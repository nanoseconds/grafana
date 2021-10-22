package setting

import (
	"path/filepath"
	"testing"

	"github.com/grafana/grafana/pkg/infra/log"
	. "github.com/smartystreets/goconvey/convey"
)

type testLogger struct {
	log.Logger
	infoCalled  bool
	infoMessage []string
}

func (stub *testLogger) Info(testMessage string, ctx ...interface{}) {
	stub.infoCalled = true
	stub.infoMessage = append(stub.infoMessage, testMessage)
}

func TestSessionSettings(t *testing.T) {
	Convey("session config", t, func() {
		skipStaticRootValidation = true

		Convey("Reading session should log error ", func() {
			cfg := NewCfg()
			homePath := "../../"

			stub := &testLogger{}
			cfg.Logger = stub

			err := cfg.Load(CommandLineArgs{
				HomePath: homePath,
				Config:   filepath.Join(homePath, "pkg/setting/testdata/session.ini"),
			})
			So(err, ShouldBeNil)

			So(stub.infoCalled, ShouldEqual, true)
			So(len(stub.infoMessage), ShouldBeGreaterThan, 1)
		})
	})
}
