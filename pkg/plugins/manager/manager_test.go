package manager

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"

	"github.com/grafana/grafana/pkg/infra/fs"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/plugins"
	"github.com/grafana/grafana/pkg/plugins/backendplugin"
	"github.com/grafana/grafana/pkg/plugins/manager/installer"
	"github.com/grafana/grafana/pkg/services/sqlstore"
	"github.com/grafana/grafana/pkg/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/ini.v1"
)

const (
	testPluginID = "test-plugin"
)

func TestPluginManager_init(t *testing.T) {
	t.Run("Plugin folder will be created if not exists", func(t *testing.T) {
		testDir := "plugin-test-dir"

		exists, err := fs.Exists(testDir)
		require.NoError(t, err)
		assert.False(t, exists)

		pm := createManager(t, func(pm *PluginManager) {
			pm.cfg.PluginsPath = testDir
		})

		err = pm.init()
		require.NoError(t, err)

		exists, err = fs.Exists(testDir)
		require.NoError(t, err)
		assert.True(t, exists)

		t.Cleanup(func() {
			err = os.Remove(testDir)
			require.NoError(t, err)
		})
	})
}

func TestPluginManager_loadPlugins(t *testing.T) {
	t.Run("Managed backend plugin", func(t *testing.T) {
		p, pc := createDSPlugin(testPluginID, "", true, true)

		loader := &fakeLoader{
			mockedLoadedPlugins: []*plugins.Plugin{p},
		}

		pm := createManager(t, func(pm *PluginManager) {
			pm.pluginLoader = loader
		})
		err := pm.loadPlugins("test/path")
		require.NoError(t, err)

		assert.Equal(t, 1, pc.startCount)
		assert.Equal(t, 0, pc.stopCount)
		assert.False(t, pc.exited)
		assert.False(t, pc.decommissioned)
		assert.Equal(t, p, pm.Plugin(testPluginID))
		assert.Len(t, pm.Plugins(), 1)

		verifyNoPluginErrors(t, pm)
	})

	t.Run("Unmanaged backend plugin", func(t *testing.T) {
		p, pc := createDSPlugin(testPluginID, "", false, true)

		loader := &fakeLoader{
			mockedLoadedPlugins: []*plugins.Plugin{p},
		}

		pm := createManager(t, func(pm *PluginManager) {
			pm.pluginLoader = loader
		})
		err := pm.loadPlugins("test/path")
		require.NoError(t, err)

		assert.Equal(t, 0, pc.startCount)
		assert.Equal(t, 0, pc.stopCount)
		assert.False(t, pc.exited)
		assert.False(t, pc.decommissioned)
		assert.Equal(t, p, pm.Plugin(testPluginID))
		assert.Len(t, pm.Plugins(), 1)

		verifyNoPluginErrors(t, pm)
	})

	t.Run("Managed non-backend plugin", func(t *testing.T) {
		p, pc := createDSPlugin(testPluginID, "", false, true)

		loader := &fakeLoader{
			mockedLoadedPlugins: []*plugins.Plugin{p},
		}

		pm := createManager(t, func(pm *PluginManager) {
			pm.pluginLoader = loader
		})
		err := pm.loadPlugins("test/path")
		require.NoError(t, err)

		assert.Equal(t, 0, pc.startCount)
		assert.Equal(t, 0, pc.stopCount)
		assert.False(t, pc.exited)
		assert.False(t, pc.decommissioned)
		assert.Equal(t, p, pm.Plugin(testPluginID))
		assert.Len(t, pm.Plugins(), 1)

		verifyNoPluginErrors(t, pm)
	})

	t.Run("Unmanaged non-backend plugin", func(t *testing.T) {
		p, pc := createDSPlugin(testPluginID, "", false, false)

		loader := &fakeLoader{
			mockedLoadedPlugins: []*plugins.Plugin{p},
		}

		pm := createManager(t, func(pm *PluginManager) {
			pm.pluginLoader = loader
		})
		err := pm.loadPlugins("test/path")
		require.NoError(t, err)

		assert.Equal(t, 0, pc.startCount)
		assert.Equal(t, 0, pc.stopCount)
		assert.False(t, pc.exited)
		assert.False(t, pc.decommissioned)
		assert.Equal(t, p, pm.Plugin(testPluginID))
		assert.Len(t, pm.Plugins(), 1)

		verifyNoPluginErrors(t, pm)
	})
}

func TestPluginManager_Run(t *testing.T) {
	t.Run("Cancelled context will cause running plugins to stop", func(t *testing.T) {

	})
}

func TestPluginManager_Installer(t *testing.T) {
	t.Run("Install plugin after manager init", func(t *testing.T) {
		i := &fakePluginInstaller{}

		p, pc := createDSPlugin(testPluginID, "1.0.0", true, true)

		l := &fakeLoader{
			mockedLoadedPlugins: []*plugins.Plugin{p},
		}

		pm := createManager(t, func(pm *PluginManager) {
			pm.pluginInstaller = i
			pm.pluginLoader = l
		})

		err := pm.Install(context.Background(), testPluginID, "1.0.0", plugins.InstallOpts{})
		require.NoError(t, err)

		assert.Equal(t, 1, i.installCount)
		assert.Equal(t, 0, i.uninstallCount)

		verifyNoPluginErrors(t, pm)

		assert.Len(t, pm.Routes(), 1)
		assert.Equal(t, p.ID, pm.Routes()[0].PluginID)
		assert.Equal(t, p.PluginDir, pm.Routes()[0].Directory)

		assert.Equal(t, 1, pc.startCount)
		assert.Equal(t, 0, pc.stopCount)
		assert.False(t, pc.exited)
		assert.False(t, pc.decommissioned)
		assert.Equal(t, p, pm.Plugin(testPluginID))
		assert.Len(t, pm.Plugins(), 1)

		t.Run("Won't install if already installed", func(t *testing.T) {
			err := pm.Install(context.Background(), testPluginID, "1.0.0", plugins.InstallOpts{})
			assert.Equal(t, plugins.DuplicatePluginError{
				PluginID:          p.ID,
				ExistingPluginDir: p.PluginDir,
			}, err)
		})

		t.Run("Uninstall base case", func(t *testing.T) {
			err := pm.Uninstall(context.Background(), p.ID)
			require.NoError(t, err)

			assert.Equal(t, 1, i.installCount)
			assert.Equal(t, 1, i.uninstallCount)

			assert.Nil(t, pm.Plugin(p.ID))
			assert.Len(t, pm.Routes(), 0)

			t.Run("Won't uninstall if not installed", func(t *testing.T) {
				err := pm.Uninstall(context.Background(), p.ID)
				require.Equal(t, plugins.ErrPluginNotInstalled, err)
			})
		})
	})
}

func TestManager(t *testing.T) {
	newManagerScenario(t, true, func(t *testing.T, ctx *managerScenarioCtx) {
		t.Run("Managed plugin scenario", func(t *testing.T) {
			t.Run("Should be able to register plugin", func(t *testing.T) {
				err := ctx.manager.registerAndStart(context.Background(), ctx.plugin)
				require.NoError(t, err)
				require.NotNil(t, ctx.plugin)
				require.Equal(t, testPluginID, ctx.plugin.ID)
				require.Equal(t, 1, ctx.pluginClient.startCount)
				require.NotNil(t, ctx.manager.Plugin(testPluginID))

				t.Run("Should not be able to register an already registered plugin", func(t *testing.T) {
					err := ctx.manager.registerAndStart(context.Background(), ctx.plugin)
					require.Equal(t, 1, ctx.pluginClient.startCount)
					require.Error(t, err)
				})

				t.Run("When manager runs should start and stop plugin", func(t *testing.T) {
					pCtx := context.Background()
					cCtx, cancel := context.WithCancel(pCtx)
					var wg sync.WaitGroup
					wg.Add(1)
					var runErr error
					go func() {
						runErr = ctx.manager.Run(cCtx)
						wg.Done()
					}()
					time.Sleep(time.Millisecond)
					cancel()
					wg.Wait()
					require.Equal(t, context.Canceled, runErr)
					require.Equal(t, 1, ctx.pluginClient.startCount)
					require.Equal(t, 1, ctx.pluginClient.stopCount)
				})

				t.Run("When manager runs should restart plugin process when killed", func(t *testing.T) {
					ctx.pluginClient.stopCount = 0
					ctx.pluginClient.startCount = 0
					pCtx := context.Background()
					cCtx, cancel := context.WithCancel(pCtx)
					var wgRun sync.WaitGroup
					wgRun.Add(1)
					var runErr error
					go func() {
						runErr = ctx.manager.Run(cCtx)
						wgRun.Done()
					}()

					time.Sleep(time.Millisecond)

					var wgKill sync.WaitGroup
					wgKill.Add(1)
					go func() {
						ctx.pluginClient.kill()
						for {
							if !ctx.plugin.Exited() {
								break
							}
						}
						cancel()
						wgKill.Done()
					}()
					wgKill.Wait()
					wgRun.Wait()
					require.Equal(t, context.Canceled, runErr)
					require.Equal(t, 1, ctx.pluginClient.stopCount)
					require.Equal(t, 1, ctx.pluginClient.startCount)
				})

				t.Run("Unimplemented handlers", func(t *testing.T) {
					t.Run("Collect metrics should return method not implemented error", func(t *testing.T) {
						_, err = ctx.manager.CollectMetrics(context.Background(), testPluginID)
						require.Equal(t, backendplugin.ErrMethodNotImplemented, err)
					})

					t.Run("Check health should return method not implemented error", func(t *testing.T) {
						_, err = ctx.manager.CheckHealth(context.Background(), backend.PluginContext{PluginID: testPluginID})
						require.Equal(t, backendplugin.ErrMethodNotImplemented, err)
					})

					t.Run("Call resource should return method not implemented error", func(t *testing.T) {
						req, err := http.NewRequest(http.MethodGet, "/test", bytes.NewReader([]byte{}))
						require.NoError(t, err)
						w := httptest.NewRecorder()
						err = ctx.manager.callResourceInternal(w, req, backend.PluginContext{PluginID: testPluginID})
						require.Equal(t, backendplugin.ErrMethodNotImplemented, err)
					})
				})

				t.Run("Implemented handlers", func(t *testing.T) {
					t.Run("Collect metrics should return expected result", func(t *testing.T) {
						ctx.pluginClient.CollectMetricsHandlerFunc = func(ctx context.Context) (*backend.CollectMetricsResult, error) {
							return &backend.CollectMetricsResult{
								PrometheusMetrics: []byte("hello"),
							}, nil
						}

						res, err := ctx.manager.CollectMetrics(context.Background(), testPluginID)
						require.NoError(t, err)
						require.NotNil(t, res)
						require.Equal(t, "hello", string(res.PrometheusMetrics))
					})

					t.Run("Check health should return expected result", func(t *testing.T) {
						json := []byte(`{
							"key": "value"
						}`)
						ctx.pluginClient.CheckHealthHandlerFunc = func(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
							return &backend.CheckHealthResult{
								Status:      backend.HealthStatusOk,
								Message:     "All good",
								JSONDetails: json,
							}, nil
						}

						res, err := ctx.manager.CheckHealth(context.Background(), backend.PluginContext{PluginID: testPluginID})
						require.NoError(t, err)
						require.NotNil(t, res)
						require.Equal(t, backend.HealthStatusOk, res.Status)
						require.Equal(t, "All good", res.Message)
						require.Equal(t, json, res.JSONDetails)
					})

					t.Run("Call resource should return expected response", func(t *testing.T) {
						ctx.pluginClient.CallResourceHandlerFunc = func(ctx context.Context,
							req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
							return sender.Send(&backend.CallResourceResponse{
								Status: http.StatusOK,
							})
						}

						req, err := http.NewRequest(http.MethodGet, "/test", bytes.NewReader([]byte{}))
						require.NoError(t, err)
						w := httptest.NewRecorder()
						err = ctx.manager.callResourceInternal(w, req, backend.PluginContext{PluginID: testPluginID})
						require.NoError(t, err)
						require.Equal(t, http.StatusOK, w.Code)
					})
				})

				t.Run("Should be able to decommission a running plugin", func(t *testing.T) {
					require.True(t, ctx.manager.isRegistered(testPluginID))

					err := ctx.manager.unregisterAndStop(context.Background(), ctx.plugin)
					require.NoError(t, err)

					require.Equal(t, 2, ctx.pluginClient.stopCount)
					require.False(t, ctx.manager.isRegistered(testPluginID))
					p := ctx.manager.plugins[testPluginID]
					require.Nil(t, p)

					err = ctx.manager.start(context.Background(), ctx.plugin)
					require.Equal(t, backendplugin.ErrPluginNotRegistered, err)
				})
			})
		})
	})

	newManagerScenario(t, false, func(t *testing.T, ctx *managerScenarioCtx) {
		t.Run("Unmanaged plugin scenario", func(t *testing.T) {
			t.Run("Should be able to register plugin", func(t *testing.T) {
				err := ctx.manager.registerAndStart(context.Background(), ctx.plugin)
				require.NoError(t, err)
				require.True(t, ctx.manager.isRegistered(testPluginID))
				require.False(t, ctx.pluginClient.managed)

				t.Run("When manager runs should not start plugin", func(t *testing.T) {
					pCtx := context.Background()
					cCtx, cancel := context.WithCancel(pCtx)
					var wg sync.WaitGroup
					wg.Add(1)
					var runErr error
					go func() {
						runErr = ctx.manager.Run(cCtx)
						wg.Done()
					}()
					go func() {
						cancel()
					}()
					wg.Wait()
					require.Equal(t, context.Canceled, runErr)
					require.Equal(t, 0, ctx.pluginClient.startCount)
					require.Equal(t, 1, ctx.pluginClient.stopCount)
					require.True(t, ctx.plugin.Exited())
				})

				t.Run("Should be not be able to start unmanaged plugin", func(t *testing.T) {
					pCtx := context.Background()
					cCtx, cancel := context.WithCancel(pCtx)
					defer cancel()
					err := ctx.manager.start(cCtx, ctx.plugin)
					require.Nil(t, err)
					require.Equal(t, 0, ctx.pluginClient.startCount)
					require.True(t, ctx.plugin.Exited())
				})
			})
		})
	})
}

func createDSPlugin(pluginID, version string, managed, backend bool) (*plugins.Plugin, *fakePluginClient) {
	p := &plugins.Plugin{
		Class: plugins.External,
		JSONData: plugins.JSONData{
			ID:      pluginID,
			Type:    plugins.DataSource,
			Backend: backend,
			Info: plugins.PluginInfo{
				Version: version,
			},
		},
	}

	logger := fakeLogger{}

	p.SetLogger(logger)

	pc := &fakePluginClient{
		pluginID: pluginID,
		logger:   logger,
		managed:  managed,
	}

	p.RegisterClient(pc)

	return p, pc
}

type fakePluginInstaller struct {
	installer.Installer

	installCount   int
	uninstallCount int
}

func (f *fakePluginInstaller) Install(ctx context.Context, pluginID, version, pluginsDir, pluginZipURL, pluginRepoURL string) error {
	f.installCount++
	return nil
}

func (f *fakePluginInstaller) Uninstall(ctx context.Context, pluginPath string) error {
	f.uninstallCount++
	return nil
}

func (f *fakePluginInstaller) GetUpdateInfo(ctx context.Context, pluginID, version, pluginRepoURL string) (plugins.UpdateInfo, error) {
	return plugins.UpdateInfo{}, nil
}

func createManager(t *testing.T, cbs ...func(*PluginManager)) *PluginManager {
	t.Helper()

	staticRootPath, err := filepath.Abs("../../../public/")
	require.NoError(t, err)

	cfg := &setting.Cfg{
		Raw:            ini.Empty(),
		Env:            setting.Prod,
		StaticRootPath: staticRootPath,
	}

	license := &fakeLicensingService{}
	requestValidator := &testPluginRequestValidator{}
	loader := &fakeLoader{}
	pm := newManager(cfg, license, requestValidator, &sqlstore.SQLStore{})
	pm.pluginLoader = loader

	for _, cb := range cbs {
		cb(pm)
	}

	return pm
}

type managerScenarioCtx struct {
	manager      *PluginManager
	plugin       *plugins.Plugin
	pluginClient *fakePluginClient
}

func newManagerScenario(t *testing.T, managed bool, fn func(t *testing.T, ctx *managerScenarioCtx)) {
	t.Helper()
	cfg := setting.NewCfg()
	cfg.AWSAllowedAuthProviders = []string{"keys", "credentials"}
	cfg.AWSAssumeRoleEnabled = true

	cfg.Azure.ManagedIdentityEnabled = true
	cfg.Azure.Cloud = "AzureCloud"
	cfg.Azure.ManagedIdentityClientId = "client-id"

	staticRootPath, err := filepath.Abs("../../../public")
	require.NoError(t, err)
	cfg.StaticRootPath = staticRootPath

	license := &fakeLicensingService{}
	requestValidator := &testPluginRequestValidator{}
	loader := &fakeLoader{}
	manager := newManager(cfg, license, requestValidator, nil)
	manager.pluginLoader = loader
	ctx := &managerScenarioCtx{
		manager: manager,
	}

	logger := fakeLogger{}

	ctx.plugin = &plugins.Plugin{
		JSONData: plugins.JSONData{
			ID:      testPluginID,
			Backend: true,
		},
	}
	ctx.plugin.SetLogger(logger)

	ctx.pluginClient = &fakePluginClient{
		pluginID: testPluginID,
		logger:   fakeLogger{},
		managed:  managed,
	}

	ctx.plugin.RegisterClient(ctx.pluginClient)

	fn(t, ctx)
}

func verifyNoPluginErrors(t *testing.T, pm *PluginManager) {
	for _, plugin := range pm.Plugins() {
		assert.Nil(t, plugin.SignatureError)
	}
}

type fakeLoader struct {
	mockedLoadedPlugins       []*plugins.Plugin
	mockedFactoryLoadedPlugin *plugins.Plugin

	loadedPaths []string

	plugins.Loader
}

func (l *fakeLoader) Load(paths []string, ignore map[string]struct{}) ([]*plugins.Plugin, error) {
	l.loadedPaths = append(l.loadedPaths, paths...)

	return l.mockedLoadedPlugins, nil
}

func (l *fakeLoader) LoadWithFactory(path string, factory backendplugin.PluginFactoryFunc) (*plugins.Plugin, error) {
	l.loadedPaths = append(l.loadedPaths, path)

	return l.mockedFactoryLoadedPlugin, nil
}

type fakePluginClient struct {
	pluginID       string
	logger         log.Logger
	startCount     int
	stopCount      int
	managed        bool
	exited         bool
	decommissioned bool
	backend.CollectMetricsHandlerFunc
	backend.CheckHealthHandlerFunc
	backend.QueryDataHandlerFunc
	backend.CallResourceHandlerFunc
	mutex sync.RWMutex

	backendplugin.Plugin
}

func (tp *fakePluginClient) PluginID() string {
	return tp.pluginID
}

func (tp *fakePluginClient) Logger() log.Logger {
	return tp.logger
}

func (tp *fakePluginClient) Start(ctx context.Context) error {
	tp.mutex.Lock()
	defer tp.mutex.Unlock()
	tp.exited = false
	tp.startCount++
	return nil
}

func (tp *fakePluginClient) Stop(ctx context.Context) error {
	tp.mutex.Lock()
	defer tp.mutex.Unlock()
	tp.stopCount++
	tp.exited = true
	return nil
}

func (tp *fakePluginClient) IsManaged() bool {
	return tp.managed
}

func (tp *fakePluginClient) Exited() bool {
	tp.mutex.RLock()
	defer tp.mutex.RUnlock()
	return tp.exited
}

func (tp *fakePluginClient) Decommission() error {
	tp.mutex.Lock()
	defer tp.mutex.Unlock()

	tp.decommissioned = true

	return nil
}

func (tp *fakePluginClient) IsDecommissioned() bool {
	tp.mutex.RLock()
	defer tp.mutex.RUnlock()
	return tp.decommissioned
}

func (tp *fakePluginClient) kill() {
	tp.mutex.Lock()
	defer tp.mutex.Unlock()
	tp.exited = true
}

func (tp *fakePluginClient) CollectMetrics(ctx context.Context) (*backend.CollectMetricsResult, error) {
	if tp.CollectMetricsHandlerFunc != nil {
		return tp.CollectMetricsHandlerFunc(ctx)
	}

	return nil, backendplugin.ErrMethodNotImplemented
}

func (tp *fakePluginClient) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	if tp.CheckHealthHandlerFunc != nil {
		return tp.CheckHealthHandlerFunc(ctx, req)
	}

	return nil, backendplugin.ErrMethodNotImplemented
}

func (tp *fakePluginClient) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	if tp.QueryDataHandlerFunc != nil {
		return tp.QueryDataHandlerFunc(ctx, req)
	}

	return nil, backendplugin.ErrMethodNotImplemented
}

func (tp *fakePluginClient) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	if tp.CallResourceHandlerFunc != nil {
		return tp.CallResourceHandlerFunc(ctx, req, sender)
	}

	return backendplugin.ErrMethodNotImplemented
}

func (tp *fakePluginClient) SubscribeStream(ctx context.Context, request *backend.SubscribeStreamRequest) (*backend.SubscribeStreamResponse, error) {
	return nil, backendplugin.ErrMethodNotImplemented
}

func (tp *fakePluginClient) PublishStream(ctx context.Context, request *backend.PublishStreamRequest) (*backend.PublishStreamResponse, error) {
	return nil, backendplugin.ErrMethodNotImplemented
}

func (tp *fakePluginClient) RunStream(ctx context.Context, request *backend.RunStreamRequest, sender *backend.StreamSender) error {
	return backendplugin.ErrMethodNotImplemented
}

type fakeLicensingService struct {
	edition    string
	hasLicense bool
	tokenRaw   string
}

func (t *fakeLicensingService) HasLicense() bool {
	return t.hasLicense
}

func (t *fakeLicensingService) Expiry() int64 {
	return 0
}

func (t *fakeLicensingService) Edition() string {
	return t.edition
}

func (t *fakeLicensingService) StateInfo() string {
	return ""
}

func (t *fakeLicensingService) ContentDeliveryPrefix() string {
	return ""
}

func (t *fakeLicensingService) LicenseURL(showAdminLicensingPage bool) string {
	return ""
}

func (t *fakeLicensingService) HasValidLicense() bool {
	return false
}

func (t *fakeLicensingService) Environment() map[string]string {
	return map[string]string{"GF_ENTERPRISE_LICENSE_TEXT": t.tokenRaw}
}

type testPluginRequestValidator struct{}

func (t *testPluginRequestValidator) Validate(string, *http.Request) error {
	return nil
}

type fakeLogger struct {
	log.Logger
}

func (tl fakeLogger) Debug(msg string, ctx ...interface{}) {

}
