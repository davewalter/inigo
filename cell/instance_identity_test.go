package cell_test

import (
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	archive_helper "code.cloudfoundry.org/archiver/extractor/test_helper"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/inigo/fixtures"
	"code.cloudfoundry.org/inigo/helpers"
	"code.cloudfoundry.org/inigo/world"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/rep/cmd/rep/config"

	"crypto/tls"
	"crypto/x509"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/onsi/gomega/matchers"
	"github.com/onsi/gomega/types"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
	"github.com/tedsuo/ifrit/grouper"
)

var _ = Describe("InstanceIdentity", func() {
	var (
		credDir                                     string
		validityPeriod                              time.Duration
		cellProcess                                 ifrit.Process
		fileServerStaticDir                         string
		intermediateCACertPath, intermediateKeyPath string
		rootCAs                                     *x509.CertPool
		client                                      http.Client
		lrp                                         *models.DesiredLRP
		processGUID                                 string
		organizationalUnit                          []string
		rep, fileServer, metronAgent                ifrit.Runner
	)

	BeforeEach(func() {
		// We can only do one OrganizationalUnit at the moment until go1.8
		// Make this 2 organizational units after we update to go1.8
		// https://github.com/golang/go/issues/18654
		organizationalUnit = []string{"jim:radical"}

		var err error
		credDir = world.TempDir("instance-creds")

		caCertPath, err := filepath.Abs("../fixtures/certs/ca-with-no-max-path-length.crt")
		Expect(err).NotTo(HaveOccurred())
		intermediateCACertPath, err = filepath.Abs("../fixtures/certs/instance-identity.crt")
		Expect(err).NotTo(HaveOccurred())
		intermediateKeyPath, err = filepath.Abs("../fixtures/certs/instance-identity.key")
		Expect(err).NotTo(HaveOccurred())
		caCertContent, err := ioutil.ReadFile(caCertPath)
		Expect(err).NotTo(HaveOccurred())
		caCert := parseCertificate(caCertContent, true)
		rootCAs = x509.NewCertPool()
		rootCAs.AddCert(caCert)

		validityPeriod = time.Minute

		configRepCerts := func(cfg *config.RepConfig) {
			cfg.InstanceIdentityCredDir = credDir
			cfg.InstanceIdentityCAPath = intermediateCACertPath
			cfg.InstanceIdentityPrivateKeyPath = intermediateKeyPath
			cfg.InstanceIdentityValidityPeriod = durationjson.Duration(validityPeriod)
		}

		exportNetworkVars := func(config *config.RepConfig) {
			config.ExportNetworkEnvVars = true
		}

		client = http.Client{}
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
				RootCAs:            rootCAs,
			},
		}

		processGUID = helpers.GenerateGuid()
		lrp = helpers.DefaultLRPCreateRequest(processGUID, "log-guid", 1)
		lrp.Setup = nil
		lrp.CachedDependencies = []*models.CachedDependency{{
			From:      fmt.Sprintf("http://%s/v1/static/%s", componentMaker.Addresses.FileServer, "lrp.zip"),
			To:        "/tmp/diego",
			Name:      "lrp bits",
			CacheKey:  "lrp-cache-key",
			LogSource: "APP",
		}}
		lrp.MetricsGuid = processGUID
		lrp.LegacyDownloadUser = "vcap"
		lrp.Privileged = true
		lrp.Action = models.WrapAction(&models.RunAction{
			User: "vcap",
			Path: "/tmp/diego/go-server",
			Env: []*models.EnvironmentVariable{
				{"PORT", "8080"},
				{"HTTPS_PORT", "8081"},
			},
		})
		lrp.CertificateProperties = &models.CertificateProperties{
			OrganizationalUnit: organizationalUnit,
		}

		fileServer, fileServerStaticDir = componentMaker.FileServer()
		archiveFiles := fixtures.GoServerApp()
		archive_helper.CreateTarGZArchive(filepath.Join(fileServerStaticDir, "lrp.tgz"), archiveFiles)
		archive_helper.CreateZipArchive(
			filepath.Join(fileServerStaticDir, "lrp.zip"),
			archiveFiles,
		)

		rep = componentMaker.Rep(configRepCerts, exportNetworkVars)
		metronAgent = ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
			close(ready)
		loop:
			for {
				select {
				case <-signals:
					logger.Info("signaled")
					break loop
				}
			}
			return nil
		})
	})

	JustBeforeEach(func() {
		cellGroup := grouper.Members{
			{"router", componentMaker.Router()},
			{"metron-agent", metronAgent},
			{"file-server", fileServer},
			{"rep", rep},
			{"auctioneer", componentMaker.Auctioneer()},
			{"route-emitter", componentMaker.RouteEmitter()},
		}
		cellProcess = ginkgomon.Invoke(grouper.NewParallel(os.Interrupt, cellGroup))

		Eventually(func() (models.CellSet, error) { return bbsServiceClient.Cells(logger) }).Should(HaveLen(1))
	})

	AfterEach(func() {
		os.RemoveAll(credDir)
		helpers.StopProcesses(cellProcess)
	})

	verifyCertAndKey := func(data []byte, organizationalUnit []string) {
		block, rest := pem.Decode(data)
		Expect(rest).NotTo(BeEmpty())
		Expect(block).NotTo(BeNil())
		containerCert := block.Bytes

		// skip the intermediate cert which is concatenated to the container cert
		block, rest = pem.Decode(rest)
		Expect(block).NotTo(BeNil())

		block, rest = pem.Decode(rest)
		Expect(rest).To(BeEmpty())
		Expect(block).NotTo(BeNil())
		containerKey := block.Bytes

		By("verify the certificate is signed properly")
		cert := parseCertificate(containerCert, false)
		Expect(cert.Subject.OrganizationalUnit).To(Equal(organizationalUnit))
		Expect(cert.NotAfter.Sub(cert.NotBefore)).To(Equal(validityPeriod))

		caCertContent, err := ioutil.ReadFile(intermediateCACertPath)
		Expect(err).NotTo(HaveOccurred())

		caCert := parseCertificate(caCertContent, true)
		verifyCertificateIsSignedBy(cert, caCert)

		By("verify the private key matches the cert public key")
		key, err := x509.ParsePKCS1PrivateKey(containerKey)
		Expect(err).NotTo(HaveOccurred())
		Expect(&key.PublicKey).To(Equal(cert.PublicKey))
	}

	verifyCertAndKeyArePresentForTask := func(certPath, keyPath string, organizationalUnit []string) {
		By("running the task and getting the concatenated pem cert and key")
		result := runTaskAndGetCommandOutput(fmt.Sprintf("cat %s %s", certPath, keyPath), organizationalUnit)
		verifyCertAndKey([]byte(result), organizationalUnit)
	}

	verifyCertAndKeyArePresentForLRP := func(ipAddress string, organizationalUnit []string) {
		resp, err := client.Get(fmt.Sprintf("https://%s:8081/cf-instance-cert", ipAddress))
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()

		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		certData, err := ioutil.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())

		resp, err = client.Get(fmt.Sprintf("https://%s:8081/cf-instance-key", ipAddress))
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()

		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		keyData, err := ioutil.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())

		data := append(certData, keyData...)
		verifyCertAndKey(data, organizationalUnit)
	}

	Context("tasks", func() {
		It("should add instance identity certificate and key in the right location", func() {
			verifyCertAndKeyArePresentForTask("/etc/cf-instance-credentials/instance.crt", "/etc/cf-instance-credentials/instance.key", organizationalUnit)
		})

		It("should add instance identity environment variables to the container", func() {
			verifyCertAndKeyArePresentForTask("$CF_INSTANCE_CERT", "$CF_INSTANCE_KEY", organizationalUnit)
		})
	})

	Context("lrps", func() {
		var ipAddress string

		JustBeforeEach(func() {
			err := bbsClient.DesireLRP(logger, lrp)
			Expect(err).NotTo(HaveOccurred())
			Eventually(helpers.LRPStatePoller(logger, bbsClient, processGUID, nil)).Should(Equal(models.ActualLRPStateRunning))

			ipAddress = getContainerInternalIP()
		})

		It("should add instance identity certificate and key in the right location", func() {
			verifyCertAndKeyArePresentForLRP(ipAddress, organizationalUnit)
		})

		It("does not write container proxy config files", func() {
			resp, err := client.Get(fmt.Sprintf("https://%s:8081/cat?file=/etc/cf-assets/envoy_config/envoy.json", ipAddress))
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})
	})

	Context("when a server uses the provided cert and key", func() {
		var ipAddress string

		JustBeforeEach(func() {
			err := bbsClient.DesireLRP(logger, lrp)
			Expect(err).NotTo(HaveOccurred())
			Eventually(helpers.LRPStatePoller(logger, bbsClient, processGUID, nil)).Should(Equal(models.ActualLRPStateRunning))

			ipAddress = getContainerInternalIP()
		})

		Context("and a client app tries to connect using the root ca cert", func() {
			It("successfully connects and verify the sever identity", func() {
				resp, err := client.Get(fmt.Sprintf("https://%s:8081/env", ipAddress))
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				defer resp.Body.Close()
				body, err := ioutil.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(body)).To(ContainSubstring("CF_INSTANCE_INTERNAL_IP=" + ipAddress))
			})
		})
	})

	Context("when a server has client authentication enabled using the root CA", func() {
		var (
			url string
		)

		JustBeforeEach(func() {
			server := ghttp.NewUnstartedServer()
			server.HTTPTestServer.TLS = &tls.Config{
				ClientCAs:  rootCAs,
				ClientAuth: tls.RequireAndVerifyClientCert,
			}
			ipAddress, err := localip.LocalIP()
			Expect(err).NotTo(HaveOccurred())
			listener, err := net.Listen("tcp4", ipAddress+":0")
			Expect(err).NotTo(HaveOccurred())
			server.AppendHandlers(ghttp.RespondWith(http.StatusOK, "hello world"))
			server.HTTPTestServer.Listener = listener
			server.HTTPTestServer.StartTLS()
			url = server.Addr()
		})

		Context("and a client app tries to connect to the server using the instance identity cert", func() {
			var (
				output string
			)

			JustBeforeEach(func() {
				output = runTaskAndGetCommandOutput(fmt.Sprintf("curl --silent -k --cert /etc/cf-instance-credentials/instance.crt --key /etc/cf-instance-credentials/instance.key https://%s", url), []string{})
			})

			It("successfully connects", func() {
				Expect(output).To(ContainSubstring("hello world"))
			})
		})
	})

	Context("when running with envoy proxy", func() {
		var ipAddress string
		var configRepCerts func(cfg *config.RepConfig)
		var exportNetworkVars func(cfg *config.RepConfig)
		var enableContainerProxy func(cfg *config.RepConfig)

		BeforeEach(func() {
			configRepCerts = func(cfg *config.RepConfig) {
				cfg.InstanceIdentityCredDir = credDir
				cfg.InstanceIdentityCAPath = intermediateCACertPath
				cfg.InstanceIdentityPrivateKeyPath = intermediateKeyPath
				cfg.InstanceIdentityValidityPeriod = durationjson.Duration(validityPeriod)
			}

			exportNetworkVars = func(config *config.RepConfig) {
				config.ExportNetworkEnvVars = true
			}

			enableContainerProxy = func(config *config.RepConfig) {
				config.EnableContainerProxy = true
				config.EnvoyConfigRefreshDelay = durationjson.Duration(time.Second)
				config.ContainerProxyPath = os.Getenv("ENVOY_PATH")

				tmpdir := world.TempDir("envoy_config")
				Expect(err).NotTo(HaveOccurred())

				config.ContainerProxyConfigPath = tmpdir
			}

			rep = componentMaker.Rep(configRepCerts, exportNetworkVars, enableContainerProxy)
		})

		JustBeforeEach(func() {
			err := bbsClient.DesireLRP(logger, lrp)
			Expect(err).NotTo(HaveOccurred())
			Eventually(helpers.LRPStatePoller(logger, bbsClient, processGUID, nil)).Should(Equal(models.ActualLRPStateRunning))

			ipAddress = getContainerInternalIP()
		})

		connect := func() error {
			resp, err := client.Get(fmt.Sprintf("https://%s:61001/env", ipAddress))
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return errors.New("not statusok")
			}

			return nil
		}

		It("should have a container with envoy enabled on it", func() {
			Eventually(connect).Should(Succeed())
		})

		Context("when the container uses OCI preloaded rootfs", func() {
			BeforeEach(func() {
				if !world.UseGrootFS() {
					Skip("Not using grootfs")
				}

				lrp.CachedDependencies = nil
				layer := fmt.Sprintf("http://%s/v1/static/%s", componentMaker.Addresses.FileServer, "lrp.tgz")
				lrp.RootFs = "preloaded+layer:" + helpers.DefaultStack + "?layer=" + layer + "&layer_path=/" + "&layer_digest="
				lrp.Action = models.WrapAction(&models.RunAction{
					User: "vcap",
					Path: "/go-server",
					Env: []*models.EnvironmentVariable{
						{"PORT", "8080"},
						{"HTTPS_PORT", "8081"},
					},
				})
			})

			It("should have a container with envoy enabled on it", func() {
				Eventually(connect).Should(Succeed())
			})
		})

		Context("when certs are rotated", func() {
			var credRotationPeriod = 64 * time.Second

			BeforeEach(func() {
				alterCredRotation := func(config *config.RepConfig) {
					config.InstanceIdentityValidityPeriod = durationjson.Duration(credRotationPeriod)
				}

				rep = componentMaker.Rep(configRepCerts, exportNetworkVars, enableContainerProxy, alterCredRotation)
			})

			It("should be able to reconnect with the updated certs", func() {
				Eventually(connect).Should(Succeed())
				Consistently(connect, 90*time.Second).Should(Succeed())
			})
		})

		Context("when the additional memory allocation is defined", func() {
			var (
				container                garden.Container
				setProxyMemoryAllocation func(cfg *config.RepConfig)
				containerMutex           sync.Mutex
			)

			BeforeEach(func() {
				containerMutex.Lock()
				defer containerMutex.Unlock()
				container = nil

				lrp.MemoryMb = 64
				setProxyMemoryAllocation = func(config *config.RepConfig) {
					config.ProxyMemoryAllocationMB = 5
				}

				rep = componentMaker.Rep(configRepCerts, exportNetworkVars, enableContainerProxy, setProxyMemoryAllocation)
			})

			JustBeforeEach(func() {
				Eventually(helpers.LRPStatePoller(logger, bbsClient, processGUID, nil)).Should(Equal(models.ActualLRPStateRunning))

				lrps, err := bbsClient.ActualLRPGroupsByProcessGuid(logger, processGUID)
				Expect(err).NotTo(HaveOccurred())

				actualLRP := lrps[0].Instance
				containerHandle := actualLRP.InstanceGuid

				containerMutex.Lock()
				defer containerMutex.Unlock()
				container, err = gardenClient.Lookup(containerHandle)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when emitting app metrics", func() {
				var (
					metricsChan chan map[string]uint64
					memoryLimit uint64
				)

				BeforeEach(func() {
					metricsChan = make(chan map[string]uint64, 10)
					memoryLimit = uint64(lrp.MemoryMb)

					addr, err := net.ResolveUDPAddr("udp", ":0")
					Expect(err).NotTo(HaveOccurred())
					udpConn, err := net.ListenUDP("udp", addr)
					Expect(err).NotTo(HaveOccurred())

					metronAgent = ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
						defer GinkgoRecover()
						defer udpConn.Close()

						logger := logger.Session("metron-agent")
						close(ready)
						logger.Info("starting", lager.Data{"port": addr.Port})
						msgs := make(chan []byte)
						errCh := make(chan error)
						go func() {
							for {
								bs := make([]byte, 102400)
								n, _, err := udpConn.ReadFromUDP(bs)
								if err != nil {
									errCh <- err
									return
								}
								msgs <- bs[:n]
							}
						}()
						for {
							select {
							case <-signals:
								logger.Info("signaled")
								return nil
							case err := <-errCh:
								return err
							case msg := <-msgs:
								var envelope events.Envelope
								err := proto.Unmarshal(msg, &envelope)
								if err != nil {
									continue
								}
								metric := envelope.GetContainerMetric()
								if metric == nil {
									continue
								}

								containerMutex.Lock()
								c := container
								containerMutex.Unlock()
								if c == nil {
									// container can be nil if we get a container metric while
									// the JustBeforeEach is still getting the actual lrp and
									// container infromation
									continue
								}

								metrics, err := c.Metrics()
								if err != nil {
									// do not return the error since garden will initially
									continue
								}
								stats := metrics.MemoryStat
								actualMemoryUsage := stats.TotalRss + stats.TotalCache - stats.TotalInactiveFile

								metricsChan <- map[string]uint64{
									"memory":        metric.GetMemoryBytes(),
									"actual_memory": actualMemoryUsage,
									"memory_quota":  metric.GetMemoryBytesQuota(),
								}
							}
						}
					})

					dropsondeConfig := func(cfg *config.RepConfig) {
						cfg.DropsondePort = udpConn.LocalAddr().(*net.UDPAddr).Port
						cfg.ContainerMetricsReportInterval = durationjson.Duration(5 * time.Second)
					}

					rep = componentMaker.Rep(
						configRepCerts, exportNetworkVars,
						enableContainerProxy, setProxyMemoryAllocation,
						dropsondeConfig,
					)

					lrp.Action.RunAction.Args = []string{"-allocate-memory-mb=30"}
				})

				It("should receive rescaled memory usage", func() {
					Eventually(metricsChan, 10*time.Second).Should(Receive(ScaledDownMemory(memoryLimit, 5)))
				})

				It("should receive rescaled memory limit", func() {
					Eventually(metricsChan, 10*time.Second).Should(Receive(HaveKeyWithValue("memory_quota", memoryInBytes(memoryLimit))))
				})

				Context("when the lrp is using docker rootfs", func() {
					BeforeEach(func() {
						lrp = helpers.DockerLRPCreateRequest(processGUID)
						lrp.MetricsGuid = processGUID
						lrp.MemoryMb = int32(memoryLimit)
					})

					It("should not receive rescaled memory limit", func() {
						Eventually(metricsChan, 10*time.Second).Should(Receive(UnscaledDownMemory()))
					})

					It("should not receive rescaled memory usage", func() {
						Eventually(metricsChan, 10*time.Second).Should(Receive(HaveKeyWithValue("memory_quota", memoryInBytes(memoryLimit))))
					})
				})
			})
		})
	})
})

func UnscaledDownMemory() types.GomegaMatcher {
	someNonZeroValue := uint64(1) // otherwise the ScaledDownMemory will attempt to divide by 0
	return ScaledDownMemory(someNonZeroValue, 0)
}

func ScaledDownMemory(memoryLimit, proxyAllocatedMemory uint64) types.GomegaMatcher {
	return And(
		HaveKey("memory"),
		HaveKey("actual_memory"),
		matchers.NewWithTransformMatcher(func(m map[string]uint64) int64 {
			return int64(m["memory"]) - int64(m["actual_memory"]*memoryLimit/(memoryLimit+proxyAllocatedMemory))
		}, BeNumerically("~", 0, 102400)),
	)
}

func memoryInBytes(memoryMb uint64) uint64 {
	return memoryMb * 1024 * 1024
}

func getContainerInternalIP() string {
	By("getting the internal ip address of the container")
	var (
		body []byte
		code int
		err  error
	)
	Eventually(func() int {
		body, code, err = helpers.ResponseBodyAndStatusCodeFromHost(componentMaker.Addresses.Router, helpers.DefaultHost, "env")
		Expect(err).NotTo(HaveOccurred())
		return code
	}).Should(Equal(http.StatusOK))
	var ipAddress string
	for _, line := range strings.Fields(string(body)) {
		if strings.HasPrefix(line, "CF_INSTANCE_INTERNAL_IP=") {
			ipAddress = strings.Split(line, "=")[1]
		}
	}
	return ipAddress
}

func runTaskAndGetCommandOutput(command string, organizationalUnits []string) string {
	guid := helpers.GenerateGuid()

	expectedTask := helpers.TaskCreateRequestWithCertificateProperties(
		guid,
		&models.RunAction{
			User: "vcap",
			Path: "sh",
			Args: []string{"-c", fmt.Sprintf("%s > thingy", command)},
		},
		&models.CertificateProperties{
			OrganizationalUnit: organizationalUnits,
		},
	)
	expectedTask.ResultFile = "/home/vcap/thingy"

	err := bbsClient.DesireTask(logger, expectedTask.TaskGuid, expectedTask.Domain, expectedTask.TaskDefinition)
	Expect(err).NotTo(HaveOccurred())

	var task *models.Task
	Eventually(func() interface{} {
		var err error

		task, err = bbsClient.TaskByGuid(logger, guid)
		Expect(err).NotTo(HaveOccurred())

		return task.State
	}).Should(Equal(models.Task_Completed))

	Expect(task.Failed).To(BeFalse())

	return task.Result
}

func parseCertificate(cert []byte, pemEncoded bool) *x509.Certificate {
	if pemEncoded {
		block, _ := pem.Decode(cert)
		Expect(block).NotTo(BeNil())
		cert = block.Bytes
	}
	certs, err := x509.ParseCertificates(cert)
	Expect(err).NotTo(HaveOccurred())
	Expect(certs).To(HaveLen(1))
	return certs[0]
}

func verifyCertificateIsSignedBy(cert, parentCert *x509.Certificate) {
	certPool := x509.NewCertPool()
	certPool.AddCert(parentCert)
	certs, err := cert.Verify(x509.VerifyOptions{
		Roots: certPool,
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(certs).To(HaveLen(1))
	Expect(certs[0]).To(ContainElement(parentCert))
}
