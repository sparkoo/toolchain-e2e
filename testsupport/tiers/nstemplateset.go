package tiers

import (
	"fmt"
	"reflect"
	"sort"
	"sync"
	"testing"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/toolchain-e2e/testsupport/wait"
	"github.com/davecgh/go-spew/spew"

	"github.com/stretchr/testify/require"
)

func VerifyNSTemplateSet(t *testing.T, hostAwait *wait.HostAwaitility, memberAwait *wait.MemberAwaitility, nsTmplSet *toolchainv1alpha1.NSTemplateSet, checks TierChecks) {
	expectedTemplateRefs := checks.GetExpectedTemplateRefs(hostAwait)

	_, err := memberAwait.WaitForNSTmplSet(nsTmplSet.Name, UntilNSTemplateSetHasTemplateRefs(expectedTemplateRefs))
	require.NoError(t, err)

	// Verify all namespaces and objects within
	namespaceObjectChecks := sync.WaitGroup{}
	for _, templateRef := range expectedTemplateRefs.Namespaces {
		ns, err := memberAwait.WaitForNamespace(nsTmplSet.Name, templateRef, nsTmplSet.Spec.TierName, wait.UntilNamespaceIsActive())
		require.NoError(t, err)
		_, nsType, _, err := wait.Split(templateRef)
		require.NoError(t, err)
		namespaceChecks := checks.GetNamespaceObjectChecks(nsType)
		for _, check := range namespaceChecks {
			namespaceObjectChecks.Add(1)
			go func(checkNamespaceObjects namespaceObjectsCheck) {
				defer namespaceObjectChecks.Done()
				checkNamespaceObjects(t, ns, memberAwait, nsTmplSet.Name)
			}(check)
		}
	}

	// Verify the Cluster Resources
	clusterObjectChecks := sync.WaitGroup{}
	if expectedTemplateRefs.ClusterResources != nil {
		clusterChecks := checks.GetClusterObjectChecks()
		for _, check := range clusterChecks {
			clusterObjectChecks.Add(1)
			go func(check clusterObjectsCheck) {
				defer clusterObjectChecks.Done()
				check(t, memberAwait, nsTmplSet.Name, nsTmplSet.Spec.TierName)
			}(check)
		}
	}
	namespaceObjectChecks.Wait()
	clusterObjectChecks.Wait()
}

// UntilNSTemplateSetHasTemplateRefs checks if the NSTemplateTier has the expected template refs
func UntilNSTemplateSetHasTemplateRefs(expectedRevisions TemplateRefs) wait.NSTemplateSetWaitCriterion {
	return wait.NSTemplateSetWaitCriterion{
		Match: func(actual *toolchainv1alpha1.NSTemplateSet) bool {
			actualNamespaces := actual.Spec.Namespaces
			if expectedRevisions.ClusterResources == nil ||
				actual.Spec.ClusterResources == nil ||
				*expectedRevisions.ClusterResources != actual.Spec.ClusterResources.TemplateRef {
				return false
			}
			actualNamespaceTmplRefs := make([]string, len(actualNamespaces))
			for i, r := range actualNamespaces {
				actualNamespaceTmplRefs[i] = r.TemplateRef
			}
			sort.Strings(actualNamespaceTmplRefs)
			sort.Strings(expectedRevisions.Namespaces)
			return reflect.DeepEqual(actualNamespaceTmplRefs, expectedRevisions.Namespaces)
		},
		Diff: func(actual *toolchainv1alpha1.NSTemplateSet) string {
			return fmt.Sprintf("expected NSTemplateSet '%s' to have the following cluster and namespace revisions: %s\nbut it contained: %s", actual.Name, spew.Sdump(expectedRevisions), spew.Sdump(actual.Spec.Namespaces))
		},
	}
}
