package provisioning

import (
	"fmt"
	"net"

	metal3v1alpha1 "github.com/openshift/cluster-baremetal-operator/pkg/apis/metal3/v1alpha1"
)

const (
	baremetalProvisioningCR        = "provisioning-configuration"
	baremetalHttpPort              = "6180"
	baremetalIronicPort            = "6385"
	baremetalIronicInspectorPort   = "5050"
	baremetalKernelUrlSubPath      = "images/ironic-python-agent.kernel"
	baremetalRamdiskUrlSubPath     = "images/ironic-python-agent.initramfs"
	baremetalIronicEndpointSubpath = "v1/"
)

// Provisioning Config needed to deploy Metal3 pod
type BaremetalProvisioningConfig struct {
	ProvisioningInterface     string
	ProvisioningIp            string
	ProvisioningNetworkCIDR   string
	ProvisioningDHCPExternal  bool
	ProvisioningDHCPRange     string
	ProvisioningOSDownloadURL string
}

func getBaremetalProvisioningConfig(cr *metal3v1alpha1.Provisioning) BaremetalProvisioningConfig {
	return BaremetalProvisioningConfig{
		ProvisioningInterface:     cr.Spec.ProvisioningInterface,
		ProvisioningIp:            cr.Spec.ProvisioningIP,
		ProvisioningNetworkCIDR:   cr.Spec.ProvisioningNetworkCIDR,
		ProvisioningDHCPExternal:  cr.Spec.ProvisioningDHCPExternal,
		ProvisioningDHCPRange:     cr.Spec.ProvisioningDHCPRange,
		ProvisioningOSDownloadURL: cr.Spec.ProvisioningOSDownloadURL,
	}
}

func getProvisioningIPCIDR(baremetalConfig BaremetalProvisioningConfig) *string {
	if baremetalConfig.ProvisioningNetworkCIDR != "" && baremetalConfig.ProvisioningIp != "" {
		_, net, err := net.ParseCIDR(baremetalConfig.ProvisioningNetworkCIDR)
		if err == nil {
			cidr, _ := net.Mask.Size()
			generatedConfig := fmt.Sprintf("%s/%d", baremetalConfig.ProvisioningIp, cidr)
			return &generatedConfig
		}
	}
	return nil
}

func getDeployKernelUrl(baremetalConfig BaremetalProvisioningConfig) *string {
	if baremetalConfig.ProvisioningIp != "" {
		generatedConfig := fmt.Sprintf("http://%s/%s", net.JoinHostPort(baremetalConfig.ProvisioningIp, baremetalHttpPort), baremetalKernelUrlSubPath)
		return &generatedConfig
	}
	return nil
}

func getDeployRamdiskUrl(baremetalConfig BaremetalProvisioningConfig) *string {
	if baremetalConfig.ProvisioningIp != "" {
		generatedConfig := fmt.Sprintf("http://%s/%s", net.JoinHostPort(baremetalConfig.ProvisioningIp, baremetalHttpPort), baremetalRamdiskUrlSubPath)
		return &generatedConfig
	}
	return nil
}

func getIronicEndpoint(baremetalConfig BaremetalProvisioningConfig) *string {
	if baremetalConfig.ProvisioningIp != "" {
		generatedConfig := fmt.Sprintf("http://%s/%s", net.JoinHostPort(baremetalConfig.ProvisioningIp, baremetalIronicPort), baremetalIronicEndpointSubpath)
		return &generatedConfig
	}
	return nil
}

func getIronicInspectorEndpoint(baremetalConfig BaremetalProvisioningConfig) *string {
	if baremetalConfig.ProvisioningIp != "" {
		generatedConfig := fmt.Sprintf("http://%s/%s", net.JoinHostPort(baremetalConfig.ProvisioningIp, baremetalIronicInspectorPort), baremetalIronicEndpointSubpath)
		return &generatedConfig
	}
	return nil
}

func getProvisioningDHCPRange(baremetalConfig BaremetalProvisioningConfig) *string {
	// When the DHCP server is external, it is OK for the DHCP range in the CR
	// to be empty.
	if baremetalConfig.ProvisioningDHCPRange != "" {
		return &(baremetalConfig.ProvisioningDHCPRange)
	} else if baremetalConfig.ProvisioningDHCPExternal {
		return &(baremetalConfig.ProvisioningDHCPRange)
	}
	return nil
}

func getProvisioningInterface(baremetalConfig BaremetalProvisioningConfig) *string {
	if baremetalConfig.ProvisioningInterface != "" {
		return &(baremetalConfig.ProvisioningInterface)
	}
	return nil
}

func getProvisioningOSDownloadURL(baremetalConfig BaremetalProvisioningConfig) *string {
	if baremetalConfig.ProvisioningOSDownloadURL != "" {
		return &(baremetalConfig.ProvisioningOSDownloadURL)
	}
	return nil
}

func getMetal3DeploymentConfig(name string, baremetalConfig BaremetalProvisioningConfig) *string {
	configValue := ""
	switch name {
	case "PROVISIONING_IP":
		return getProvisioningIPCIDR(baremetalConfig)
	case "PROVISIONING_INTERFACE":
		return getProvisioningInterface(baremetalConfig)
	case "DEPLOY_KERNEL_URL":
		return getDeployKernelUrl(baremetalConfig)
	case "DEPLOY_RAMDISK_URL":
		return getDeployRamdiskUrl(baremetalConfig)
	case "IRONIC_ENDPOINT":
		return getIronicEndpoint(baremetalConfig)
	case "IRONIC_INSPECTOR_ENDPOINT":
		return getIronicInspectorEndpoint(baremetalConfig)
	case "HTTP_PORT":
		configValue = baremetalHttpPort
		return &configValue
	case "DHCP_RANGE":
		return getProvisioningDHCPRange(baremetalConfig)
	case "RHCOS_IMAGE_URL":
		return getProvisioningOSDownloadURL(baremetalConfig)
	}
	return nil
}
