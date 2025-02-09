// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package scheme

import (
	"fmt"

	routev1 "github.com/openshift/api/route/v1"
	mchv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	clusterv1alpha1 "open-cluster-management.io/api/cluster/v1alpha1"
	clusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	clusterv1beta2 "open-cluster-management.io/api/cluster/v1beta2"
	operatorv1 "open-cluster-management.io/api/operator/v1"
	policyv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	channelv1 "open-cluster-management.io/multicloud-operators-channel/pkg/apis/apps/v1"
	placementrulev1 "open-cluster-management.io/multicloud-operators-subscription/pkg/apis/apps/placementrule/v1"
	appsubv1 "open-cluster-management.io/multicloud-operators-subscription/pkg/apis/apps/v1"
	appsubv1alpha1 "open-cluster-management.io/multicloud-operators-subscription/pkg/apis/apps/v1alpha1"
	appv1beta1 "sigs.k8s.io/application/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

// AddToScheme adds all the resources to be processed to the Scheme.
func AddToScheme(runtimeScheme *runtime.Scheme) error {
	addToSchemeFuncs := []func(s *runtime.Scheme) error{
		clusterv1.AddToScheme,
		clusterv1alpha1.AddToScheme,
		clusterv1beta1.AddToScheme,
		clusterv1beta2.AddToScheme,
		operatorv1.AddToScheme,
		apiregistrationv1.AddToScheme,
		routev1.AddToScheme,
	}

	schemeBuilders := []*scheme.Builder{
		mchv1.SchemeBuilder,
		policyv1.SchemeBuilder,
		placementrulev1.SchemeBuilder,
		appsubv1alpha1.SchemeBuilder,
		channelv1.SchemeBuilder,
		appsubv1.SchemeBuilder,
		appv1beta1.SchemeBuilder,
	}

	for _, addToSchemeFunc := range addToSchemeFuncs {
		if err := addToSchemeFunc(runtimeScheme); err != nil {
			return fmt.Errorf("failed to install scheme: %w", err)
		}
	}

	for _, schemeBuilder := range schemeBuilders {
		if err := schemeBuilder.AddToScheme(runtimeScheme); err != nil {
			return fmt.Errorf("failed to add scheme: %w", err)
		}
	}

	return nil
}
