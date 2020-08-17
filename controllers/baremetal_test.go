package provisioning

import (
	"testing"

	metal3v1alpha1 "github.com/openshift/cluster-baremetal-operator/pkg/apis/metal3/v1alpha1"
)

var provisioningCR = &metal3v1alpha1.Provisioning{
	Spec: metal3v1alpha1.ProvisioningSpec{
		ProvisioningInterface:     "ensp0",
		ProvisioningIP:            "172.30.20.3",
		ProvisioningNetworkCIDR:   "172.30.20.0/24",
		ProvisioningDHCPExternal:  false,
		ProvisioningDHCPRange:     "172.30.20.11, 172.30.20.101",
		ProvisioningOSDownloadURL: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
	},
}
var (
	expectedProvisioningInterface    = "ensp0"
	expectedProvisioningIP           = "172.30.20.3"
	expectedProvisioningNetworkCIDR  = "172.30.20.0/24"
	expectedProvisioningDHCPExternal = false
	expectedProvisioningDHCPRange    = "172.30.20.11, 172.30.20.101"
	expectedOSImageURL               = "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234"
	expectedProvisioningIPCIDR       = "172.30.20.3/24"
	expectedDeployKernelURL          = "http://172.30.20.3:6180/images/ironic-python-agent.kernel"
	expectedDeployRamdiskURL         = "http://172.30.20.3:6180/images/ironic-python-agent.initramfs"
	expectedIronicEndpoint           = "http://172.30.20.3:6385/v1/"
	expectedIronicInspectorEndpoint  = "http://172.30.20.3:5050/v1/"
	expectedHttpPort                 = "6180"
)

func TestGenerateRandomPassword(t *testing.T) {
	pwd := generateRandomPassword()
	if pwd == "" {
		t.Errorf("Expected a valid string but got null")
	}
}

// Testing the mariadb password creation
func TestCreateMariadbPasswordSecret(t *testing.T) {
	operatorConfig := &OperatorConfig{TargetNamespace: "test-namespace"}

	// First create a mariadb password secret
	oldMariadbPassword := createMariadbPasswordSecret(operatorConfig)
	oldPassword, ok := oldMariadbPassword.StringData[baremetalSecretKey]
	if !ok || oldPassword == "" {
		t.Fatal("Failure reading first Mariadb password from Secret.")
	}

	// Create another mariadb password secret
	newMariadbPassword := createMariadbPasswordSecret(operatorConfig)
	newPassword, ok := newMariadbPassword.StringData[baremetalSecretKey]
	if !ok || newPassword == "" {
		t.Fatal("Failure reading second Mariadb password from Secret.")
	}
	if oldPassword == newPassword {
		t.Fatalf("Both passwords match.")
	}
}

func TestGetBaremetalProvisioningConfig(t *testing.T) {
	baremetalConfig := getBaremetalProvisioningConfig(provisioningCR)
	if baremetalConfig.ProvisioningInterface != expectedProvisioningInterface ||
		baremetalConfig.ProvisioningIp != expectedProvisioningIP ||
		baremetalConfig.ProvisioningNetworkCIDR != expectedProvisioningNetworkCIDR ||
		baremetalConfig.ProvisioningDHCPExternal != expectedProvisioningDHCPExternal ||
		baremetalConfig.ProvisioningDHCPRange != expectedProvisioningDHCPRange {
		t.Logf("Expected: ProvisioningInterface: %s, ProvisioningIP: %s, ProvisioningNetworkCIDR: %s, ProvisioningDHCPExternal: %t, expectedProvisioningDHCPRange: %s, Got: %+v", expectedProvisioningInterface, expectedProvisioningIP, expectedProvisioningNetworkCIDR, expectedProvisioningDHCPExternal, expectedProvisioningDHCPRange, baremetalConfig)
		t.Fatalf("failed getBaremetalProvisioningConfig. One or more BaremetalProvisioningConfig items do not match the expected config.")
	}
}

func TestGetMetal3DeploymentConfig(t *testing.T) {
	baremetalConfig := getBaremetalProvisioningConfig(provisioningCR)

	actualCacheURL := getMetal3DeploymentConfig("CACHEURL", baremetalConfig)
	if actualCacheURL != nil {
		t.Errorf("CacheURL is found to be %s. CACHEURL is not expected.", *actualCacheURL)
	} else {
		t.Logf("CacheURL is not available as expected.")
	}
	actualOSImageURL := getMetal3DeploymentConfig("RHCOS_IMAGE_URL", baremetalConfig)
	if actualOSImageURL != nil {
		t.Logf("Actual OS Image Download URL is %s, Expected is %s", *actualOSImageURL, expectedOSImageURL)
		if *actualOSImageURL != expectedOSImageURL {
			t.Errorf("Actual %s and Expected %s OS Image Download URLs do not match", *actualOSImageURL, expectedOSImageURL)
		}
	} else {
		t.Errorf("OS Image Download URL is not available.")
	}
	actualProvisioningIPCIDR := getMetal3DeploymentConfig("PROVISIONING_IP", baremetalConfig)
	if actualProvisioningIPCIDR != nil {
		t.Logf("Actual ProvisioningIP with CIDR is %s, Expected is %s", *actualProvisioningIPCIDR, expectedProvisioningIPCIDR)
		if *actualProvisioningIPCIDR != expectedProvisioningIPCIDR {
			t.Errorf("Actual %s and Expected %s Provisioning IPs with CIDR do not match", *actualProvisioningIPCIDR, expectedProvisioningIPCIDR)
		}
	} else {
		t.Errorf("Provisioning IP with CIDR is not available.")
	}
	actualProvisioningInterface := getMetal3DeploymentConfig("PROVISIONING_INTERFACE", baremetalConfig)
	if actualProvisioningInterface != nil {
		t.Logf("Actual Provisioning Interface is %s, Expected is %s", *actualProvisioningInterface, expectedProvisioningInterface)
		if *actualProvisioningInterface != expectedProvisioningInterface {
			t.Errorf("Actual %s and Expected %s Provisioning Interfaces do not match", *actualProvisioningIPCIDR, expectedProvisioningIPCIDR)
		}
	} else {
		t.Errorf("Provisioning Interface is not available.")
	}
	actualDeployKernelURL := getMetal3DeploymentConfig("DEPLOY_KERNEL_URL", baremetalConfig)
	if actualDeployKernelURL != nil {
		t.Logf("Actual Deploy Kernel URL is %s, Expected is %s", *actualDeployKernelURL, expectedDeployKernelURL)
		if *actualDeployKernelURL != expectedDeployKernelURL {
			t.Errorf("Actual %s and Expected %s Deploy Kernel URLs do not match", *actualDeployKernelURL, expectedDeployKernelURL)
		}
	} else {
		t.Errorf("Deploy Kernel URL is not available.")
	}
	actualDeployRamdiskURL := getMetal3DeploymentConfig("DEPLOY_RAMDISK_URL", baremetalConfig)
	if actualDeployRamdiskURL != nil {
		t.Logf("Actual Deploy Ramdisk URL is %s, Expected is %s", *actualDeployRamdiskURL, expectedDeployRamdiskURL)
		if *actualDeployRamdiskURL != expectedDeployRamdiskURL {
			t.Errorf("Actual %s and Expected %s Deploy Ramdisk URLs do not match", *actualDeployRamdiskURL, expectedDeployRamdiskURL)
		}
	} else {
		t.Errorf("Deploy Ramdisk URL is not available.")
	}
	actualIronicEndpoint := getMetal3DeploymentConfig("IRONIC_ENDPOINT", baremetalConfig)
	if actualIronicEndpoint != nil {
		t.Logf("Actual Ironic Endpoint is %s, Expected is %s", *actualIronicEndpoint, expectedIronicEndpoint)
		if *actualIronicEndpoint != expectedIronicEndpoint {
			t.Errorf("Actual %s and Expected %s Ironic Endpoints do not match", *actualIronicEndpoint, expectedIronicEndpoint)
		}
	} else {
		t.Errorf("Ironic Endpoint is not available.")
	}
	actualIronicInspectorEndpoint := getMetal3DeploymentConfig("IRONIC_INSPECTOR_ENDPOINT", baremetalConfig)
	if actualIronicInspectorEndpoint != nil {
		t.Logf("Actual Ironic Inspector Endpoint is %s, Expected is %s", *actualIronicInspectorEndpoint, expectedIronicInspectorEndpoint)
		if *actualIronicInspectorEndpoint != expectedIronicInspectorEndpoint {
			t.Errorf("Actual %s and Expected %s Ironic Inspector Endpoints do not match", *actualIronicInspectorEndpoint, expectedIronicInspectorEndpoint)
		}
	} else {
		t.Errorf("Ironic Inspector Endpoint is not available.")
	}
	actualHttpPort := getMetal3DeploymentConfig("HTTP_PORT", baremetalConfig)
	t.Logf("Actual Http Port is %s, Expected is %s", *actualHttpPort, expectedHttpPort)
	if *actualHttpPort != expectedHttpPort {
		t.Errorf("Actual %s and Expected %s Http Ports do not match", *actualHttpPort, expectedHttpPort)
	}
	actualDHCPRange := getMetal3DeploymentConfig("DHCP_RANGE", baremetalConfig)
	if actualDHCPRange != nil {
		t.Logf("Actual DHCP Range is %s, Expected is %s", *actualDHCPRange, expectedProvisioningDHCPRange)
		if *actualDHCPRange != expectedProvisioningDHCPRange {
			t.Errorf("Actual %s and Expected %s DHCP Range do not match", *actualDHCPRange, expectedProvisioningDHCPRange)
		}
	} else {
		t.Errorf("Provisioning DHCP Range is not available.")
	}
}
