package v1alpha3

import (
	"github.com/metal-stack/cluster-api-provider-metalstack/api/v1alpha4"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

func (src *MetalStackCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha4.MetalStackCluster)
	return Convert_v1alpha3_MetalStackCluster_To_v1alpha4_MetalStackCluster(src, dst, nil)
}

func (dst *MetalStackCluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha4.MetalStackCluster)
	return Convert_v1alpha4_MetalStackCluster_To_v1alpha3_MetalStackCluster(src, dst, nil)
}

func (src *MetalStackClusterList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha4.MetalStackClusterList)
	return Convert_v1alpha3_MetalStackClusterList_To_v1alpha4_MetalStackClusterList(src, dst, nil)
}

func (dst *MetalStackClusterList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha4.MetalStackClusterList)
	return Convert_v1alpha4_MetalStackClusterList_To_v1alpha3_MetalStackClusterList(src, dst, nil)
}

func (src *MetalStackFirewall) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha4.MetalStackFirewall)
	return Convert_v1alpha3_MetalStackFirewall_To_v1alpha4_MetalStackFirewall(src, dst, nil)
}

func (dst *MetalStackFirewall) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha4.MetalStackFirewall)
	return Convert_v1alpha4_MetalStackFirewall_To_v1alpha3_MetalStackFirewall(src, dst, nil)
}

func (src *MetalStackFirewallList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha4.MetalStackFirewallList)
	return Convert_v1alpha3_MetalStackFirewallList_To_v1alpha4_MetalStackFirewallList(src, dst, nil)
}

func (dst *MetalStackFirewallList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha4.MetalStackFirewallList)
	return Convert_v1alpha4_MetalStackFirewallList_To_v1alpha3_MetalStackFirewallList(src, dst, nil)
}

func (src *MetalStackMachine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha4.MetalStackMachine)
	return Convert_v1alpha3_MetalStackMachine_To_v1alpha4_MetalStackMachine(src, dst, nil)
}

func (dst *MetalStackMachine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha4.MetalStackMachine)
	return Convert_v1alpha4_MetalStackMachine_To_v1alpha3_MetalStackMachine(src, dst, nil)
}

func (src *MetalStackMachineList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha4.MetalStackMachineList)
	return Convert_v1alpha3_MetalStackMachineList_To_v1alpha4_MetalStackMachineList(src, dst, nil)
}

func (dst *MetalStackMachineList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha4.MetalStackMachineList)
	return Convert_v1alpha4_MetalStackMachineList_To_v1alpha3_MetalStackMachineList(src, dst, nil)
}

func (src *MetalStackMachineTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha4.MetalStackMachineTemplate)
	return Convert_v1alpha3_MetalStackMachineTemplate_To_v1alpha4_MetalStackMachineTemplate(src, dst, nil)
}

func (dst *MetalStackMachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha4.MetalStackMachineTemplate)
	return Convert_v1alpha4_MetalStackMachineTemplate_To_v1alpha3_MetalStackMachineTemplate(src, dst, nil)
}

func (src *MetalStackMachineTemplateList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1alpha4.MetalStackMachineTemplateList)
	return Convert_v1alpha3_MetalStackMachineTemplateList_To_v1alpha4_MetalStackMachineTemplateList(src, dst, nil)
}

func (dst *MetalStackMachineTemplateList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1alpha4.MetalStackMachineTemplateList)
	return Convert_v1alpha4_MetalStackMachineTemplateList_To_v1alpha3_MetalStackMachineTemplateList(src, dst, nil)
}
