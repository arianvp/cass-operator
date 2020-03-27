package v1beta1

import (
	"encoding/json"
	"fmt"

	"github.com/Jeffail/gabs"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/datastax/cass-operator/operator/pkg/serverconfig"
	"github.com/datastax/cass-operator/operator/pkg/utils"
)

const (
	// For now we will define defaults for both server image types
	defaultCassRepository = "datastaxlabs/apache-cassandra-with-mgmtapi"
	defaultCassVersion    = "3.11.6-20200316"

	defaultDseRepository = "datastaxlabs/dse-k8s-server"
	defaultDseVersion    = "6.8.0-20200316"

	defaultConfigBuilderImage = "datastaxlabs/dse-k8s-config-builder:0.9.0-20200316"

	// ClusterLabel is the operator's label for the cluster name
	ClusterLabel = "cassandra.datastax.com/cluster"

	// DatacenterLabel is the operator's label for the datacenter name
	DatacenterLabel = "cassandra.datastax.com/datacenter"

	// SeedNodeLabel is the operator's label for the seed node state
	SeedNodeLabel = "cassandra.datastax.com/seed-node"

	// RackLabel is the operator's label for the rack name
	RackLabel = "cassandra.datastax.com/rack"

	// RackLabel is the operator's label for the rack name
	CassOperatorProgressLabel = "cassandra.datastax.com/operator-progress"

	// CassNodeState
	CassNodeState = "cassandra.datastax.com/node-state"

	// Progress states for status
	ProgressUpdating ProgressState = "Updating"
	ProgressReady    ProgressState = "Ready"
)

// this type exists so there's no chance of pushing random strings to our progress status
type ProgressState string

// getImageForServerVersion tries to look up a known image for a server type and version number.
// In the event that no image is found, an error is returned
func getImageForServerVersion(server, version string) (string, error) {
	sv := server + "-" + version
	switch sv {
	case "dse-6.8.0":
		return fmt.Sprintf("%s:%s", defaultDseRepository, defaultDseVersion), nil
	case "cassandra-3.11.6":
		return fmt.Sprintf("%s:%s", defaultCassRepository, defaultCassVersion), nil
	}
	err := fmt.Errorf("server '%s' and version '%s' do not work together", server, version)
	return "", err
}

// CassandraDatacenterSpec defines the desired state of CassandraDatacenter
// +k8s:openapi-gen=true
type CassandraDatacenterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags:
	// https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// Desired number of server nodes
	// +kubebuilder:validation:Minimum=1
	Size int32 `json:"size"`

	// Version number
	// +kubebuilder:validation:Enum="6.8.0";"3.11.6"
	ServerVersion string `json:"serverVersion"`

	// Server image name.
	// More info: https://kubernetes.io/docs/concepts/containers/images
	ServerImage string `json:"serverImage,omitempty"`

	// Server type: "cassandra" or "dse"
	// +kubebuilder:validation:Enum=cassandra;dse
	ServerType string `json:"serverType"`

	// Config for the server, in YAML format
	Config json.RawMessage `json:"config,omitempty"`
	// Config for the Management API certificates
	ManagementApiAuth ManagementApiAuthConfig `json:"managementApiAuth,omitempty"`
	// Kubernetes resource requests and limits, per pod
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// A list of the named racks in the datacenter, representing independent failure domains. The
	// number of racks should match the replication factor in the keyspaces you plan to create, and
	// the number of racks cannot easily be changed once a datacenter is deployed.
	Racks []Rack `json:"racks,omitempty"`
	// Describes the persistent storage request of each server node
	StorageConfig StorageConfig `json:"storageConfig"`

	// A list of pod names that need to be replaced.
	ReplaceNodes []string `json:"replaceNodes,omitempty"`

	// The name by which CQL clients and instances will know the cluster. If the same
	// cluster name is shared by multiple Datacenters in the same Kubernetes namespace,
	// they will join together in a multi-datacenter cluster.
	// +kubebuilder:validation:MinLength=2
	ClusterName string `json:"clusterName"`

	// A stopped CassandraDatacenter will have no running server pods, like using "stop" with
	// traditional System V init scripts. Other Kubernetes resources will be left intact, and volumes
	// will re-attach when the CassandraDatacenter workload is resumed.
	Stopped bool `json:"stopped,omitempty"`

	// Container image for the config builder init container, with host, path, and tag
	ConfigBuilderImage string `json:"configBuilderImage,omitempty"`

	// Indicates configuration and container image changes should only be pushed to
	// the first rack of the datacenter
	CanaryUpgrade bool `json:"canaryUpgrade,omitempty"`
	// Turning this option on allows multiple server pods to be created on a k8s worker node.
	// By default the operator creates just one server pod per k8s worker node using k8s
	// podAntiAffinity and requiredDuringSchedulingIgnoredDuringExecution.
	AllowMultipleNodesPerWorker bool `json:"allowMultipleNodesPerWorker,omitempty"`
	// This secret defines the username and password for the Server superuser.
	SuperuserSecretName string `json:"superuserSecretName,omitempty"`
	// The k8s service account to use for the server pods
	ServiceAccount string `json:"serviceAccount,omitempty"`

	// Whether to do a rolling restart at the next opportunity. The operator will set this back
	// to false once the restart is in progress.
	RollingRestartRequested bool `json:"rollingRestartRequested,omitempty"`
}

type StorageConfig struct {
	CassandraDataVolumeClaimSpec *corev1.PersistentVolumeClaimSpec `json:"cassandraDataVolumeClaimSpec,omitempty"`
}

// GetRacks is a getter for the Rack slice in the spec
// It ensures there is always at least one rack
// FIXME move this onto the CassandraDatacenter for consistency?
func (s *CassandraDatacenterSpec) GetRacks() []Rack {
	if len(s.Racks) >= 1 {
		return s.Racks
	}

	return []Rack{{
		Name: "default",
	}}
}

// Rack ...
type Rack struct {
	// The rack name
	// +kubebuilder:validation:MinLength=2
	Name string `json:"name"`
	// Zone name to pin the rack, using node affinity
	Zone string `json:"zone,omitempty"`
}

type CassandraNodeStatus struct {
	HostID string `json:"hostID,omitempty"`
	NodeIP string `json:"nodeIP,omitempty"`
}

type CassandraStatusMap map[string]CassandraNodeStatus

// CassandraDatacenterStatus defines the observed state of CassandraDatacenter
// +k8s:openapi-gen=true
type CassandraDatacenterStatus struct {
	// The timestamp at which CQL superuser credentials
	// were last upserted to the management API
	// +optional
	SuperUserUpserted metav1.Time `json:"superUserUpserted,omitempty"`

	// The timestamp when the operator last started a Server node
	// with the management API
	// +optional
	LastServerNodeStarted metav1.Time `json:"lastServerNodeStarted,omitempty"`

	// Last known progress state of the Cassandra Operator
	// +optional
	CassandraOperatorProgress ProgressState `json:"cassandraOperatorProgress,omitempty"`

	// +optional
	LastRollingRestart metav1.Time `json:"lastRollingRestart,omitempty"`

	// +optional
	NodeStatuses CassandraStatusMap `json:"nodeStatuses"`

	// +optional
	NodeReplacements []string `json:"nodeReplacements"`

	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CassandraDatacenter is the Schema for the cassandradatacenters API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=cassandradatacenters,scope=Namespaced,shortName=cassdc;cassdcs
type CassandraDatacenter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CassandraDatacenterSpec   `json:"spec,omitempty"`
	Status CassandraDatacenterStatus `json:"status,omitempty"`
}

type ManagementApiAuthManualConfig struct {
	ClientSecretName string `json:"clientSecretName"`
	ServerSecretName string `json:"serverSecretName"`
	// +optional
	SkipSecretValidation bool `json:"skipSecretValidation,omitempty"`
}

type ManagementApiAuthInsecureConfig struct {
}

type ManagementApiAuthConfig struct {
	Insecure *ManagementApiAuthInsecureConfig `json:"insecure,omitempty"`
	Manual   *ManagementApiAuthManualConfig   `json:"manual,omitempty"`
	// other strategy configs (e.g. Cert Manager) go here
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CassandraDatacenterList contains a list of CassandraDatacenter
type CassandraDatacenterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CassandraDatacenter `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CassandraDatacenter{}, &CassandraDatacenterList{})
}

func (dc *CassandraDatacenter) GetConfigBuilderImage() string {
	if dc.Spec.ConfigBuilderImage == "" {
		return defaultConfigBuilderImage
	}
	return dc.Spec.ConfigBuilderImage
}

// GetServerImage produces a fully qualified container image to pull
// based on either the version, or an explicitly specified image
//
// In the event that no valid image could be retrieved from the specified version,
// an error is returned.
func (dc *CassandraDatacenter) GetServerImage() (string, error) {
	return makeImage(dc.Spec.ServerType, dc.Spec.ServerVersion, dc.Spec.ServerImage)
}

// makeImage takes the server type/version and image from the spec,
// and returns a docker pullable server container image
// serverVersion should be a semver-like string
// serverImage should be an empty string, or [hostname[:port]/][path/with/repo]:[Server container img tag]
// If serverImage is empty, we attempt to find an appropriate container image based on the serverVersion
// In the event that no image is found, an error is returned
func makeImage(serverType, serverVersion, serverImage string) (string, error) {
	if serverImage == "" {
		return getImageForServerVersion(serverType, serverVersion)
	}
	return serverImage, nil
}

// GetRackLabels ...
func (dc *CassandraDatacenter) GetRackLabels(rackName string) map[string]string {
	labels := map[string]string{
		RackLabel: rackName,
	}

	utils.MergeMap(labels, dc.GetDatacenterLabels())

	return labels
}

// GetDatacenterLabels ...
func (dc *CassandraDatacenter) GetDatacenterLabels() map[string]string {
	labels := map[string]string{
		DatacenterLabel: dc.Name,
	}

	utils.MergeMap(labels, dc.GetClusterLabels())

	return labels
}

// GetClusterLabels returns a new map with the cluster label key and cluster name value
func (dc *CassandraDatacenter) GetClusterLabels() map[string]string {
	return map[string]string{
		ClusterLabel: dc.Spec.ClusterName,
	}
}

func (dc *CassandraDatacenter) GetSeedServiceName() string {
	return dc.Spec.ClusterName + "-seed-service"
}

func (dc *CassandraDatacenter) GetAllPodsServiceName() string {
	return dc.Spec.ClusterName + "-" + dc.Name + "-all-pods-service"
}

func (dc *CassandraDatacenter) GetDatacenterServiceName() string {
	return dc.Spec.ClusterName + "-" + dc.Name + "-service"
}

func (dc *CassandraDatacenter) ShouldGenerateSuperuserSecret() bool {
	return len(dc.Spec.SuperuserSecretName) == 0
}

func (dc *CassandraDatacenter) GetSuperuserSecretNamespacedName() types.NamespacedName {
	name := dc.Spec.ClusterName + "-superuser"
	namespace := dc.ObjectMeta.Namespace
	if len(dc.Spec.SuperuserSecretName) > 0 {
		name = dc.Spec.SuperuserSecretName
	}

	return types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
}

// GetConfigAsJSON gets a JSON-encoded string suitable for passing to configBuilder
func (dc *CassandraDatacenter) GetConfigAsJSON() (string, error) {

	// We use the cluster seed-service name here for the seed list as it will
	// resolve to the seed nodes. This obviates the need to update the
	// cassandra.yaml whenever the seed nodes change.
	modelValues := serverconfig.GetModelValues([]string{dc.GetSeedServiceName()}, dc.Spec.ClusterName, dc.Name)

	var modelBytes []byte

	modelBytes, err := json.Marshal(modelValues)
	if err != nil {
		return "", err
	}

	// Combine the model values with the user-specified values

	modelParsed, err := gabs.ParseJSON([]byte(modelBytes))
	if err != nil {
		return "", errors.Wrap(err, "Model information for CassandraDatacenter resource was not properly configured")
	}

	if dc.Spec.Config != nil {
		configParsed, err := gabs.ParseJSON([]byte(dc.Spec.Config))
		if err != nil {
			return "", errors.Wrap(err, "Error parsing Spec.Config for CassandraDatacenter resource")
		}

		if err := modelParsed.Merge(configParsed); err != nil {
			return "", errors.Wrap(err, "Error merging Spec.Config for CassandraDatacenter resource")
		}
	}

	return modelParsed.String(), nil
}

// GetContainerPorts will return the container ports for the pods in a statefulset based on the provided config
func (dc *CassandraDatacenter) GetContainerPorts() ([]corev1.ContainerPort, error) {
	ports := []corev1.ContainerPort{
		{
			// Note: Port Names cannot be more than 15 characters
			Name:          "native",
			ContainerPort: 9042,
		},
		{
			Name:          "inter-node-msg",
			ContainerPort: 8609,
		},
		{
			Name:          "intra-node",
			ContainerPort: 7000,
		},
		{
			Name:          "tls-intra-node",
			ContainerPort: 7001,
		},
		// jmx-port 7199 was here, seems like we no longer need to expose it
		{
			Name:          "mgmt-api-http",
			ContainerPort: 8080,
		},
	}

	config, err := dc.GetConfigAsJSON()
	if err != nil {
		return nil, err
	}

	var f interface{}
	err = json.Unmarshal([]byte(config), &f)
	if err != nil {
		return nil, err
	}

	m := f.(map[string]interface{})
	promConf := utils.SearchMap(m, "10-write-prom-conf")
	if _, ok := promConf["enabled"]; ok {
		ports = append(ports, corev1.ContainerPort{
			Name:          "prometheus",
			ContainerPort: 9103,
		})
	}

	return ports, nil
}
