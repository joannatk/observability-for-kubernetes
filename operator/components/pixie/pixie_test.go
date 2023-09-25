package pixie

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	wf "github.com/wavefronthq/observability-for-kubernetes/operator/api/v1alpha1"
	"github.com/wavefronthq/observability-for-kubernetes/operator/components/test"
	"github.com/wavefronthq/observability-for-kubernetes/operator/internal/testhelper/wftest"
	"github.com/wavefronthq/observability-for-kubernetes/operator/internal/util"
)

var ComponentDir = os.DirFS(filepath.Join("..", DeployDir))

func TestNewPixieComponent(t *testing.T) {
	t.Run("create config hash", func(t *testing.T) {
		config := validComponentConfig()
		t.Log(os.Getwd())

		component, err := NewComponent(ComponentDir, config)

		require.NoError(t, err)
		require.NotNil(t, component)
	})
}

func TestValidate(t *testing.T) {
	t.Run("valid component config", func(t *testing.T) {
		config := validComponentConfig()
		component, _ := NewComponent(ComponentDir, config)
		result := component.Validate()
		require.True(t, result.IsValid())
	})

	t.Run("empty disabled component config is valid", func(t *testing.T) {
		config := ComponentConfig{Enable: false}
		component, err := NewComponent(ComponentDir, config)
		result := component.Validate()
		require.NoError(t, err)
		require.True(t, result.IsValid())
	})

	t.Run("empty enabled component config is not valid", func(t *testing.T) {
		config := ComponentConfig{Enable: true}
		component, err := NewComponent(ComponentDir, config)
		result := component.Validate()
		require.NoError(t, err)
		require.False(t, result.IsValid())
	})

	t.Run("empty controller manager uid is not valid", func(t *testing.T) {
		config := validComponentConfig()
		config.ControllerManagerUID = ""
		component, err := NewComponent(ComponentDir, config)
		result := component.Validate()
		require.NoError(t, err)
		require.False(t, result.IsValid())
		require.Equal(t, "pixie: missing controller manager uid", result.Message())
	})

	t.Run("empty cluster uuid is not valid", func(t *testing.T) {
		config := validComponentConfig()
		config.ClusterUUID = ""
		component, err := NewComponent(ComponentDir, config)
		result := component.Validate()
		require.NoError(t, err)
		require.False(t, result.IsValid())
		require.Equal(t, "pixie: missing cluster uuid", result.Message())
	})

	t.Run("empty cluster name is not valid", func(t *testing.T) {
		config := validComponentConfig()
		config.ClusterName = ""
		component, err := NewComponent(ComponentDir, config)
		result := component.Validate()
		require.NoError(t, err)
		require.False(t, result.IsValid())
		require.Equal(t, "pixie: missing cluster name", result.Message())
	})

	t.Run("no pem resources set is not valid", func(t *testing.T) {
		config := validComponentConfig()
		config.PemResources = wf.Resources{}
		component, err := NewComponent(ComponentDir, config)
		result := component.Validate()
		require.NoError(t, err)
		require.False(t, result.IsValid())
		require.Equal(t, "pixie: [invalid vizier-pem.resources.limits.memory must be set, invalid vizier-pem.resources.limits.cpu must be set]", result.Message())
	})

}

func TestResources(t *testing.T) {
	t.Run("default configuration", func(t *testing.T) {
		component, _ := NewComponent(ComponentDir, validComponentConfig())
		toApply, toDelete, err := component.Resources()

		require.NoError(t, err)
		require.NotEmpty(t, toApply)
		require.Empty(t, toDelete)

		// check all resources for component labels
		test.RequireCommonLabels(t, toApply, "wavefront", "pixie", util.Namespace)

		// cluster name configmap
		configmap, err := test.GetAppliedConfigMap("pl-cloud-config", toApply)
		require.NoError(t, err)
		require.Equal(t, component.config.ClusterName, configmap.Data["PL_CLUSTER_NAME"])

		secret, err := test.GetAppliedSecret("pl-cluster-secrets", toApply)
		require.NoError(t, err)
		require.Equal(t, component.config.ClusterName, secret.StringData["cluster-name"])
		require.Equal(t, component.config.ClusterUUID, secret.StringData["cluster-id"])
	})

	t.Run("OpAppsOptimization is enabled", func(t *testing.T) {
		config := validComponentConfig()
		config.EnableOpAppsOptimization = true
		component, _ := NewComponent(ComponentDir, config)
		toApply, _, err := component.Resources()

		require.NoError(t, err)

		// vizier pem daemon set
		ds, err := test.GetAppliedDaemonSet(util.PixieVizierPEMName, toApply)
		require.NoError(t, err)
		require.Equal(t, "150", test.GetENVValue("PL_TABLE_STORE_DATA_LIMIT_MB", ds.Spec.Template.Spec.Containers[0].Env))
		require.Equal(t, "90", test.GetENVValue("PL_TABLE_STORE_HTTP_EVENTS_PERCENT", ds.Spec.Template.Spec.Containers[0].Env))
		require.Equal(t, "kTracers", test.GetENVValue("PL_STIRLING_SOURCES", ds.Spec.Template.Spec.Containers[0].Env))
	})

	t.Run("OpAppsOptimization is not enabled", func(t *testing.T) {
		config := validComponentConfig()
		config.EnableOpAppsOptimization = false
		component, _ := NewComponent(ComponentDir, config)
		toApply, _, err := component.Resources()

		require.NoError(t, err)

		// vizier pem daemon set
		ds, err := test.GetAppliedDaemonSet(util.PixieVizierPEMName, toApply)
		require.NoError(t, err)
		require.False(t, test.ENVVarExists("PL_TABLE_STORE_DATA_LIMIT_MB", ds.Spec.Template.Spec.Containers[0].Env))
		require.False(t, test.ENVVarExists("PL_TABLE_STORE_HTTP_EVENTS_PERCENT", ds.Spec.Template.Spec.Containers[0].Env))
		require.False(t, test.ENVVarExists("PL_STIRLING_SOURCES", ds.Spec.Template.Spec.Containers[0].Env))
	})

	t.Run("PemResources are set correctly", func(t *testing.T) {
		config := validComponentConfig()
		config.PemResources.Requests.Memory = "500Mi"
		config.PemResources.Requests.CPU = "50Mi"
		config.PemResources.Limits.Memory = "1Gi"
		config.PemResources.Limits.CPU = "100Mi"

		component, _ := NewComponent(ComponentDir, config)
		toApply, _, err := component.Resources()

		require.NoError(t, err)

		// vizier pem daemon set
		ds, err := test.GetAppliedDaemonSet(util.PixieVizierPEMName, toApply)
		require.NoError(t, err)
		require.Equal(t, "500Mi", ds.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().String())
		require.Equal(t, "50Mi", ds.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().String())
		require.Equal(t, "1Gi", ds.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().String())
		require.Equal(t, "100Mi", ds.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().String())
	})
}

func validComponentConfig() ComponentConfig {
	return ComponentConfig{
		Enable:                   true,
		ControllerManagerUID:     "controller-manager-uid",
		ClusterUUID:              "cluster-uuid",
		ClusterName:              wftest.DefaultClusterName,
		EnableOpAppsOptimization: true,
		PemResources: wf.Resources{Limits: wf.Resource{
			CPU:    "100Mi",
			Memory: "1Gi",
		}},
	}
}