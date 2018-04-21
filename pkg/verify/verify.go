/*
Copyright 2018 The OpenEBS Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package verify

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/AmitKumarDas/litmus/pkg/kubectl"
	"github.com/ghodss/yaml"
)

// VerifyFile type defines a yaml file path that represents an installation
// and is used for various verification purposes
type VerifyFile string

// Condition type defines a condition that can be applied against a component
// or a set of components
type Condition string

const (
	// UniqueNodeCond is a condition to check uniqueness of node
	UniqueNodeCond Condition = "unique-node"
)

// Action type defines a action that can be applied against a component
// or a set of components
type Action string

const (
	// DeleteAnyPodAction is an action to delete any pod
	DeleteAnyPodAction Action = "delete-any-pod"
	// DeleteOldestPodAction is an action to delete the oldest pod
	DeleteOldestPodAction Action = "delete-oldest-pod"
)

// DeployVerifier provides contract(s) i.e. method signature(s) to evaluate
// if an installation was deployed successfully
type DeployVerifier interface {
	IsDeployed() (yes bool, err error)
}

// ConnectVerifier provides contract(s) i.e. method signature(s) to evaluate
// if a connection is possible or not
type ConnectVerifier interface {
	IsConnected() (yes bool, err error)
}

// RunVerifier provides contract(s) i.e. method signature(s) to evaluate
// if an entity is in a running state or not
type RunVerifier interface {
	IsRunning() (yes bool, err error)
}

// ConditionVerifier provides contract(s) i.e. method signature(s) to evaluate
// if specific entities passes the condition
type ConditionVerifier interface {
	IsCondition(alias string, condition Condition) (yes bool, err error)
}

// ActionVerifier provides contract(s) i.e. method signature(s) to evaluate
// if specific entities passes the action
type ActionVerifier interface {
	IsAction(alias string, action Action) (yes bool, err error)
}

// DeployRunVerifier provides contract(s) i.e. method signature(s) to
// evaluate:
//
// 1/ if an entity is deployed &,
// 2/ if the entity is running
type DeployRunVerifier interface {
	// DeployVerifier will check if the instance has been deployed or not
	DeployVerifier
	// RunVerifier will check if the instance is in a running state or not
	RunVerifier
}

// AllVerifier provides contract(s) i.e. method signature(s) to
// evaluate:
//
// 1/ if an entity is deployed,
// 2/ if the entity is running,
// 3/ if the entity satisfies the provided condition &
// 4/ if the entity satisfies the provided action
type AllVerifier interface {
	// DeployVerifier will check if the instance has been deployed or not
	DeployVerifier
	// RunVerifier will check if the instance is in a running state or not
	RunVerifier
	// ConditionVerifier will check if the instance satisfies the provided
	// condition
	ConditionVerifier
	// ActionVerifier will check if the instance satisfies the provided action
	ActionVerifier
}

// Installation represents a set of components that represent an installation
// e.g. an operator represented by its CRDs, RBACs and Deployments forms an
// installation
type Installation struct {
	// VerifyID is an identifier that is used to tie together related installations
	// meant to be verified
	VerifyID string `json:"verifyID"`
	// Version of this installation, operator etc
	Version string `json:"version"`
	// Components of this installation
	Components []Component `json:"components"`
}

// Component is the information about a particular component
// e.g. a Kubernetes Deployment, or a Kubernetes Pod, etc can be
// a component in the overall installation
type Component struct {
	// Name of the component
	Name string `json:"name"`
	// Namespace of the component
	Namespace string `json:"namespace"`
	// Kind name of the component
	// e.g. pods, deployments, services, etc
	Kind string `json:"kind"`
	// APIVersion of the component
	APIVersion string `json:"apiVersion"`
	// Labels of the component that is used for filtering the components
	//
	// Following are some valid sample values for labels:
	//
	//    labels: name=app
	//    labels: name=app,env=prod
	Labels string `json:"labels"`
	// Alias provides a user understood description used for filtering the
	// components. This is a single word setting.
	//
	// NOTE:
	//  Ensure unique alias values in an installation
	Alias string `json:"alias"`
}

// unmarshal takes the raw yaml data and unmarshals it into Installation
func unmarshal(data []byte) (installation *Installation, err error) {
	installation = &Installation{}

	err = yaml.Unmarshal(data, installation)
	return
}

// load converts a verify file into an instance of *Installation
func load(file VerifyFile) (installation *Installation, err error) {
	if len(file) == 0 {
		err = fmt.Errorf("failed to load: verify file is not provided")
		return
	}

	d, err := ioutil.ReadFile(string(file))
	if err != nil {
		return
	}

	return unmarshal(d)
}

// KubeInstallVerify provides methods that handles verification related logic of
// an installation within kubernetes e.g. application, deployment, operator, etc
type KubeInstallVerify struct {
	// installation is the set of components that determine the install
	installation *Installation
	// kubectl enables execution of kubernetes operations
	kubectl kubectl.KubeRunner
}

// NewKubeInstallVerify provides a new instance of NewKubeInstallVerify based on
// the provided kubernetes runner & verify file
func NewKubeInstallVerify(runner kubectl.KubeRunner, file VerifyFile) (*KubeInstallVerify, error) {
	i, err := load(file)
	if err != nil {
		return nil, err
	}

	return &KubeInstallVerify{
		kubectl:      runner,
		installation: i,
	}, nil
}

// IsDeployed evaluates if all components of the installation are deployed
func (v *KubeInstallVerify) IsDeployed() (yes bool, err error) {
	if v.installation == nil {
		err = fmt.Errorf("failed to check IsDeployed: installation object is nil")
		return
	}

	for _, component := range v.installation.Components {
		yes, err = v.isComponentDeployed(component)
		if err != nil {
			break
		}
	}

	return
}

// IsRunning evaluates if all components of the installation are running
func (v *KubeInstallVerify) IsRunning() (yes bool, err error) {
	if v.installation == nil {
		err = fmt.Errorf("failed to check IsRunning: installation object is nil")
		return
	}

	for _, component := range v.installation.Components {
		yes, err = v.isPodComponentRunning(component)
		if err != nil {
			break
		}
	}

	return
}

// IsCondition evaluates if specific components satisfies the condition
func (v *KubeInstallVerify) IsCondition(alias string, condition Condition) (yes bool, err error) {
	switch condition {
	case UniqueNodeCond:
		return v.isEachComponentOnUniqueNode(alias)
	default:
		err = fmt.Errorf("condition '%s' is not supported", condition)
	}
	return
}

// IsAction evaluates if specific components satisfies the action
func (v *KubeInstallVerify) IsAction(alias string, action Action) (yes bool, err error) {
	switch action {
	case DeleteAnyPodAction:
		return v.isDeleteAnyRunningPod(alias)
	case DeleteOldestPodAction:
		return v.isDeleteOldestRunningPod(alias)
	default:
		err = fmt.Errorf("action '%s' is not supported", action)
	}
	return
}

// isDeleteAnyPod deletes a pod based on the alias
func (v *KubeInstallVerify) isDeleteAnyRunningPod(alias string) (yes bool, err error) {
	var pods = []string{}

	c, err := v.getMatchingPodComponent(alias)
	if err != nil {
		return
	}

	if len(strings.TrimSpace(c.Labels)) == 0 {
		err = fmt.Errorf("unable to fetch component '%s' '%s': component labels are missing '%s'", c.Kind, alias)
		return
	}

	pods, err = kubectl.GetRunningPods(v.kubectl, c.Namespace, c.Labels)
	if err != nil {
		return
	}

	if len(pods) == 0 {
		err = fmt.Errorf("failed to delete any running pod: pods with alias '%s' and running state are not found", alias)
		return
	}

	// delete any running pod
	err = kubectl.DeletePod(v.kubectl, pods[0], c.Namespace)
	if err != nil {
		return
	}

	yes = true
	return
}

// isDeleteOldestRunningPod deletes the oldeset pod based on the alias
func (v *KubeInstallVerify) isDeleteOldestRunningPod(alias string) (yes bool, err error) {
	var pod string

	c, err := v.getMatchingPodComponent(alias)
	if err != nil {
		return
	}

	// check for presence of labels
	if len(strings.TrimSpace(c.Labels)) == 0 {
		err = fmt.Errorf("unable to fetch component '%s' '%s': component labels are missing '%s'", c.Kind, alias)
		return
	}

	// fetch oldest running pod
	pod, err = kubectl.GetOldestRunningPod(v.kubectl, c.Namespace, c.Labels)
	if err != nil {
		return
	}

	if len(pod) == 0 {
		err = fmt.Errorf("failed to delete oldest running pod: pod with alias '%s' and running state is not found", alias)
		return
	}

	// delete oldest running pod
	err = kubectl.DeletePod(v.kubectl, pod, c.Namespace)
	if err != nil {
		return
	}

	yes = true
	return
}

func (v *KubeInstallVerify) getMatchingPodComponent(alias string) (comp Component, err error) {
	var filtered = []Component{}

	// filter the components that are pods & match with the provided alias
	for _, c := range v.installation.Components {
		if c.Alias == alias && kubectl.IsPod(c.Kind) {
			filtered = append(filtered, c)
		}
	}

	if len(filtered) == 0 {
		err = fmt.Errorf("component not found for alias '%s'", alias)
		return
	}

	// there should be only one component that matches the alias
	if len(filtered) > 1 {
		err = fmt.Errorf("multiple components found for alias '%s': alias should be unique in an install", alias)
		return
	}

	return filtered[0], nil
}

// isComponentDeployed flags if a particular component is deployed
func (v *KubeInstallVerify) isComponentDeployed(component Component) (yes bool, err error) {
	return kubectl.IsResourceDeployed(v.kubectl, component.Kind, component.Name, component.Namespace, component.Labels)
}

// isPodComponentRunning flags if a particular component is running
func (v *KubeInstallVerify) isPodComponentRunning(component Component) (yes bool, err error) {
	if len(strings.TrimSpace(component.Kind)) == 0 {
		err = fmt.Errorf("unable to verify component running status: component kind is required")
	}

	// return true for non pod components
	if !kubectl.IsPod(component.Kind) {
		yes = true
		return
	}

	// if pod then verify if its running
	if len(strings.TrimSpace(component.Labels)) == 0 {
		err = fmt.Errorf("unable to verify component '%s' running status: component labels are required", component.Kind)
		return
	}
	return kubectl.ArePodsRunning(v.kubectl, component.Namespace, component.Labels)
}

// isEachComponentOnUniqueNode flags if each component is placed on unique node
func (v *KubeInstallVerify) isEachComponentOnUniqueNode(alias string) (bool, error) {
	var filtered = []Component{}
	var nodes = []string{}

	// filter the components based on the provided alias
	for _, c := range v.installation.Components {
		if c.Alias == alias {
			filtered = append(filtered, c)
		}
	}

	// get the node of each filtered component
	for _, f := range filtered {
		// skip for non pod components
		if !kubectl.IsPod(f.Kind) {
			continue
		}

		// if pod then get the node on which it is running
		if len(strings.TrimSpace(f.Labels)) == 0 {
			return false, fmt.Errorf("unable to fetch component '%s' node: component labels are required", f.Kind)
		}

		n, err := kubectl.GetPodNodes(v.kubectl, f.Namespace, f.Labels)
		if err != nil {
			return false, err
		}

		nodes = append(nodes, n...)
	}

	// check if condition is satisfied i.e. no duplicate nodes
	exists := map[string]string{}
	for _, n := range nodes {
		if _, ok := exists[n]; ok {
			return false, nil
		}
		exists[n] = "tracked"
	}

	return true, nil
}

// KubeConnectionVerify provides methods that verifies connection to a kubernetes
// environment
type KubeConnectionVerify struct {
	// kubectl enables execution of kubernetes operations
	kubectl kubectl.KubeRunner
}

// NewKubeConnectionVerify provides a new instance of KubeConnectionVerify based on the provided
// kubernetes runner
func NewKubeConnectionVerify(runner kubectl.KubeRunner) *KubeConnectionVerify {
	return &KubeConnectionVerify{
		kubectl: runner,
	}
}

// IsConnected verifies if kubectl can connect to the target Kubernetes cluster
func (k *KubeConnectionVerify) IsConnected() (yes bool, err error) {
	_, err = k.kubectl.Run([]string{"get", "pods"}, "", "")
	if err == nil {
		yes = true
	}
	return
}
