package bootstrap

import (
	"testing"

	"github.com/GnosiST/platform-go/internal/platform/config"
	"github.com/GnosiST/platform-go/internal/platform/dataprotection"
)

func testDataProtectionRuntime(t *testing.T) dataprotection.Runtime {
	t.Helper()
	runtime, err := DataProtectionRuntimeFromConfig(dataProtectionConfigForTest(config.RuntimeEnvironmentTest, dataprotection.ProviderLocalTest))
	if err != nil {
		t.Fatal(err)
	}
	return runtime
}
