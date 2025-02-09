package deployer

import (
	"context"
	"encoding/json"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Deployer is the interface for the kubernetes resource deployer
type Deployer interface {
	Deploy(unsObj *unstructured.Unstructured) error
}

type deployFunc func(*unstructured.Unstructured, *unstructured.Unstructured) error

// HoHDeployer is an implementation of Deployer interface
type HoHDeployer struct {
	client      client.Client
	deployFuncs map[string]deployFunc
}

// NewHoHDeployer creates a new HoHDeployer
func NewHoHDeployer(client client.Client) Deployer {
	deployer := &HoHDeployer{client: client}
	deployer.deployFuncs = map[string]deployFunc{
		"Deployment":         deployer.deployDeployment,
		"StatefulSet":        deployer.deployStatefulSet,
		"Service":            deployer.deployService,
		"ServiceAccount":     deployer.deployServiceAccount,
		"ConfigMap":          deployer.deployConfigMap,
		"Secret":             deployer.deploySecret,
		"Role":               deployer.deployRole,
		"RoleBinding":        deployer.deployRoleBinding,
		"ClusterRole":        deployer.deployClusterRole,
		"ClusterRoleBinding": deployer.deployClusterRoleBinding,
	}
	return deployer
}

func (d *HoHDeployer) Deploy(unsObj *unstructured.Unstructured) error {
	foundObj := &unstructured.Unstructured{}
	foundObj.SetGroupVersionKind(unsObj.GetObjectKind().GroupVersionKind())
	err := d.client.Get(
		context.TODO(),
		types.NamespacedName{Name: unsObj.GetName(), Namespace: unsObj.GetNamespace()},
		foundObj,
	)
	if err != nil {
		if errors.IsNotFound(err) {
			return d.client.Create(context.TODO(), unsObj)
		}
		return err
	}

	// if resource has annotation skip-creation-if-exist: true, then it will not be updated
	metadata, ok := unsObj.Object["metadata"].(map[string]interface{})
	if ok {
		annotations, ok := metadata["annotations"].(map[string]interface{})
		if ok && annotations != nil && annotations["skip-creation-if-exist"] != nil {
			if strings.ToLower(annotations["skip-creation-if-exist"].(string)) == "true" {
				return nil
			}
		}
	}

	deployFunction, ok := d.deployFuncs[foundObj.GetKind()]
	if ok {
		return deployFunction(unsObj, foundObj)
	} else {
		if !apiequality.Semantic.DeepDerivative(unsObj, foundObj) {
			unsObj.SetGroupVersionKind(unsObj.GetObjectKind().GroupVersionKind())
			unsObj.SetResourceVersion(foundObj.GetResourceVersion())
			return d.client.Update(context.TODO(), unsObj)
		}
	}

	return nil
}

func (d *HoHDeployer) deployDeployment(desiredObj, existingObj *unstructured.Unstructured) error {
	existingJSON, _ := existingObj.MarshalJSON()
	existingDepoly := &appsv1.Deployment{}
	err := json.Unmarshal(existingJSON, existingDepoly)
	if err != nil {
		return err
	}

	desiredJSON, _ := desiredObj.MarshalJSON()
	desiredDepoly := &appsv1.Deployment{}
	err = json.Unmarshal(desiredJSON, desiredDepoly)
	if err != nil {
		return err
	}

	if !apiequality.Semantic.DeepDerivative(desiredDepoly.Spec, existingDepoly.Spec) ||
		!apiequality.Semantic.DeepDerivative(desiredDepoly.GetLabels(), existingDepoly.GetLabels()) ||
		!apiequality.Semantic.DeepDerivative(desiredDepoly.GetAnnotations(), existingDepoly.GetAnnotations()) {
		return d.client.Update(context.TODO(), desiredDepoly)
	}

	return nil
}

func (d *HoHDeployer) deployStatefulSet(desiredObj, existingObj *unstructured.Unstructured) error {
	existingJSON, _ := existingObj.MarshalJSON()
	existingSTS := &appsv1.StatefulSet{}
	err := json.Unmarshal(existingJSON, existingSTS)
	if err != nil {
		return err
	}

	desiredJSON, _ := desiredObj.MarshalJSON()
	desiredSTS := &appsv1.StatefulSet{}
	err = json.Unmarshal(desiredJSON, desiredSTS)
	if err != nil {
		return err
	}

	if !apiequality.Semantic.DeepDerivative(desiredSTS.Spec, existingSTS.Spec) ||
		!apiequality.Semantic.DeepDerivative(desiredSTS.GetLabels(), existingSTS.GetLabels()) ||
		!apiequality.Semantic.DeepDerivative(desiredSTS.GetAnnotations(), existingSTS.GetAnnotations()) {
		return d.client.Update(context.TODO(), desiredSTS)
	}

	return nil
}

func (d *HoHDeployer) deployService(desiredObj, existingObj *unstructured.Unstructured) error {
	existingJSON, _ := existingObj.MarshalJSON()
	existingService := &corev1.Service{}
	err := json.Unmarshal(existingJSON, existingService)
	if err != nil {
		return err
	}

	desiredJSON, _ := desiredObj.MarshalJSON()
	desiredService := &corev1.Service{}
	err = json.Unmarshal(desiredJSON, desiredService)
	if err != nil {
		return err
	}

	if !apiequality.Semantic.DeepDerivative(desiredService.Spec, existingService.Spec) ||
		!apiequality.Semantic.DeepDerivative(desiredService.GetLabels(), existingService.GetLabels()) ||
		!apiequality.Semantic.DeepDerivative(desiredService.GetAnnotations(), existingService.GetAnnotations()) {
		desiredService.ObjectMeta.ResourceVersion = existingService.ObjectMeta.ResourceVersion
		desiredService.Spec.ClusterIP = existingService.Spec.ClusterIP
		return d.client.Update(context.TODO(), desiredService)
	}

	return nil
}

func (d *HoHDeployer) deployServiceAccount(desiredObj, existingObj *unstructured.Unstructured) error {
	existingJSON, _ := existingObj.MarshalJSON()
	existingSA := &corev1.ServiceAccount{}
	err := json.Unmarshal(existingJSON, existingSA)
	if err != nil {
		return err
	}

	desiredJSON, _ := desiredObj.MarshalJSON()
	desiredSA := &corev1.ServiceAccount{}
	err = json.Unmarshal(desiredJSON, desiredSA)
	if err != nil {
		return err
	}

	if !apiequality.Semantic.DeepDerivative(desiredSA.Secrets, existingSA.Secrets) ||
		!apiequality.Semantic.DeepDerivative(desiredSA.ImagePullSecrets, existingSA.ImagePullSecrets) ||
		!apiequality.Semantic.DeepDerivative(desiredSA.GetLabels(), existingSA.GetLabels()) ||
		!apiequality.Semantic.DeepDerivative(desiredSA.GetAnnotations(), existingSA.GetAnnotations()) {
		return d.client.Update(context.TODO(), desiredSA)
	}

	return nil
}

func (d *HoHDeployer) deployConfigMap(desiredObj, existingObj *unstructured.Unstructured) error {
	existingJSON, _ := existingObj.MarshalJSON()
	existingConfigMap := &corev1.ConfigMap{}
	err := json.Unmarshal(existingJSON, existingConfigMap)
	if err != nil {
		return err
	}

	desiredJSON, _ := desiredObj.MarshalJSON()
	desiredConfigMap := &corev1.ConfigMap{}
	err = json.Unmarshal(desiredJSON, desiredConfigMap)
	if err != nil {
		return err
	}

	if !apiequality.Semantic.DeepDerivative(desiredConfigMap.Data, existingConfigMap.Data) ||
		!apiequality.Semantic.DeepDerivative(desiredConfigMap.GetLabels(), existingConfigMap.GetLabels()) ||
		!apiequality.Semantic.DeepDerivative(desiredConfigMap.GetAnnotations(), existingConfigMap.GetAnnotations()) {
		return d.client.Update(context.TODO(), desiredConfigMap)
	}

	return nil
}

func (d *HoHDeployer) deploySecret(desiredObj, existingObj *unstructured.Unstructured) error {
	existingJSON, _ := existingObj.MarshalJSON()
	existingSecret := &corev1.Secret{}
	err := json.Unmarshal(existingJSON, existingSecret)
	if err != nil {
		return err
	}

	desiredJSON, _ := desiredObj.MarshalJSON()
	desiredSecret := &corev1.Secret{}
	err = json.Unmarshal(desiredJSON, desiredSecret)
	if err != nil {
		return err
	}

	// handle secret stringData and data
	existingStrData := map[string]string{}
	for key, value := range existingSecret.Data {
		existingStrData[key] = string(value)
	}

	if !apiequality.Semantic.DeepDerivative(desiredSecret.StringData, existingStrData) ||
		!apiequality.Semantic.DeepDerivative(desiredSecret.Data, existingSecret.Data) ||
		!apiequality.Semantic.DeepDerivative(desiredSecret.GetLabels(), existingSecret.GetLabels()) ||
		!apiequality.Semantic.DeepDerivative(desiredSecret.GetAnnotations(), existingSecret.GetAnnotations()) {
		return d.client.Update(context.TODO(), desiredSecret)
	}

	return nil
}

func (d *HoHDeployer) deployRole(desiredObj, existingObj *unstructured.Unstructured) error {
	existingJSON, _ := existingObj.MarshalJSON()
	existingRole := &rbacv1.Role{}
	err := json.Unmarshal(existingJSON, existingRole)
	if err != nil {
		return err
	}

	desiredJSON, _ := desiredObj.MarshalJSON()
	desiredRole := &rbacv1.Role{}
	err = json.Unmarshal(desiredJSON, desiredRole)
	if err != nil {
		return err
	}

	if !apiequality.Semantic.DeepDerivative(desiredRole.Rules, existingRole.Rules) ||
		!apiequality.Semantic.DeepDerivative(desiredRole.GetLabels(), existingRole.GetLabels()) ||
		!apiequality.Semantic.DeepDerivative(desiredRole.GetAnnotations(), existingRole.GetAnnotations()) {
		return d.client.Update(context.TODO(), desiredRole)
	}

	return nil
}

func (d *HoHDeployer) deployRoleBinding(desiredObj, existingObj *unstructured.Unstructured) error {
	existingJSON, _ := existingObj.MarshalJSON()
	existingRB := &rbacv1.RoleBinding{}
	err := json.Unmarshal(existingJSON, existingRB)
	if err != nil {
		return err
	}

	desiredJSON, _ := desiredObj.MarshalJSON()
	desiredRB := &rbacv1.RoleBinding{}
	err = json.Unmarshal(desiredJSON, desiredRB)
	if err != nil {
		return err
	}

	if !apiequality.Semantic.DeepDerivative(desiredRB.Subjects, existingRB.Subjects) ||
		!apiequality.Semantic.DeepDerivative(desiredRB.RoleRef, existingRB.RoleRef) ||
		!apiequality.Semantic.DeepDerivative(desiredRB.GetLabels(), existingRB.GetLabels()) ||
		!apiequality.Semantic.DeepDerivative(desiredRB.GetAnnotations(), existingRB.GetAnnotations()) {
		return d.client.Update(context.TODO(), desiredRB)
	}

	return nil
}

func (d *HoHDeployer) deployClusterRole(desiredObj, existingObj *unstructured.Unstructured) error {
	existingJSON, _ := existingObj.MarshalJSON()
	existingCB := &rbacv1.ClusterRole{}
	err := json.Unmarshal(existingJSON, existingCB)
	if err != nil {
		return err
	}

	desiredJSON, _ := desiredObj.MarshalJSON()
	desiredCB := &rbacv1.ClusterRole{}
	err = json.Unmarshal(desiredJSON, desiredCB)
	if err != nil {
		return err
	}

	if !apiequality.Semantic.DeepDerivative(desiredCB.Rules, existingCB.Rules) ||
		!apiequality.Semantic.DeepDerivative(desiredCB.AggregationRule, existingCB.AggregationRule) ||
		!apiequality.Semantic.DeepDerivative(desiredCB.GetLabels(), existingCB.GetLabels()) ||
		!apiequality.Semantic.DeepDerivative(desiredCB.GetAnnotations(), existingCB.GetAnnotations()) {
		return d.client.Update(context.TODO(), desiredCB)
	}

	return nil
}

func (d *HoHDeployer) deployClusterRoleBinding(desiredObj, existingObj *unstructured.Unstructured) error {
	existingJSON, _ := existingObj.MarshalJSON()
	existingCRB := &rbacv1.ClusterRoleBinding{}
	err := json.Unmarshal(existingJSON, existingCRB)
	if err != nil {
		return err
	}

	desiredJSON, _ := desiredObj.MarshalJSON()
	desiredCRB := &rbacv1.ClusterRoleBinding{}
	err = json.Unmarshal(desiredJSON, desiredCRB)
	if err != nil {
		return err
	}

	if !apiequality.Semantic.DeepDerivative(desiredCRB.Subjects, existingCRB.Subjects) ||
		!apiequality.Semantic.DeepDerivative(desiredCRB.RoleRef, existingCRB.RoleRef) ||
		!apiequality.Semantic.DeepDerivative(desiredCRB.GetLabels(), existingCRB.GetLabels()) ||
		!apiequality.Semantic.DeepDerivative(desiredCRB.GetAnnotations(), existingCRB.GetAnnotations()) {
		return d.client.Update(context.TODO(), desiredCRB)
	}

	return nil
}
