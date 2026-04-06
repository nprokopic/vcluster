// Suite: lifecycle
// Tests tenant cluster lifecycle (create, list, delete, pause, resume).
// No shared vCluster needed — tests manage their own instances.
// Run: ginkgo -v --focus='Tenant cluster lifecycle' ./e2e-next
package e2e_next

import (
	_ "github.com/loft-sh/vcluster/e2e-next/test_core/lifecycle"
)
