/*
Copyright 2022 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webhook

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
)

const (
	SidecarContainerName            = "gke-gcsfuse-sidecar"
	SidecarContainerVolumeName      = "gke-gcsfuse-tmp"
	SidecarContainerVolumeMountPath = "/tmp"
)

func GetSidecarContainerSpec(c *Config) v1.Container {
	return v1.Container{
		Name:            SidecarContainerName,
		Image:           c.ContainerImage + ":" + c.ImageVersion,
		ImagePullPolicy: v1.PullPolicy(c.ImagePullPolicy),
		SecurityContext: &v1.SecurityContext{
			AllowPrivilegeEscalation: func(b bool) *bool { return &b }(false),
			RunAsUser:                func(i int64) *int64 { return &i }(0),
			RunAsGroup:               func(i int64) *int64 { return &i }(0),
		},
		Args: []string{"--v=5"},
		Resources: v1.ResourceRequirements{
			Limits: v1.ResourceList{
				v1.ResourceCPU:              c.CPULimit,
				v1.ResourceMemory:           c.MemoryLimit,
				v1.ResourceEphemeralStorage: c.EphemeralStorageLimit,
			},
			Requests: v1.ResourceList{
				v1.ResourceCPU:              c.CPULimit,
				v1.ResourceMemory:           c.MemoryLimit,
				v1.ResourceEphemeralStorage: c.EphemeralStorageLimit,
			},
		},
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      SidecarContainerVolumeName,
				MountPath: SidecarContainerVolumeMountPath,
			},
		},
	}
}

func GetSidecarContainerVolumeSpec() v1.Volume {
	return v1.Volume{
		Name: SidecarContainerVolumeName,
		VolumeSource: v1.VolumeSource{
			EmptyDir: &v1.EmptyDirVolumeSource{},
		},
	}
}

// ValidatePodHasSidecarContainerInjected validates the following:
// 1. One of the container name matches the sidecar container name.
// 2. The image name matches.
// 3. The container has a volume with the sidecar container volume name.
// 4. The volume has the sidecar container volume mount path.
// 5. The Pod has an emptyDir volume with the sidecar container volume name.
func ValidatePodHasSidecarContainerInjected(image string, pod *corev1.Pod) bool {
	containerInjected := false
	volumeInjected := false
	for _, c := range pod.Spec.Containers {
		if c.Name == SidecarContainerName && strings.Split(c.Image, ":")[0] == image {
			for _, v := range c.VolumeMounts {
				if v.Name == SidecarContainerVolumeName && v.MountPath == SidecarContainerVolumeMountPath {
					containerInjected = true
					break
				}
			}
			break
		}
	}

	for _, v := range pod.Spec.Volumes {
		if v.Name == SidecarContainerVolumeName && v.VolumeSource.EmptyDir != nil {
			volumeInjected = true
			break
		}
	}
	return containerInjected && volumeInjected
}