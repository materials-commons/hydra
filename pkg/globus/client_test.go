package globus

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var testEndpointID string

func TestClient_GetIdentities(t *testing.T) {
	client := createClient(t)
	identities, err := client.GetIdentities([]string{"glenn.tarcea@gmail.com"})
	assert.NoErrorf(t, err, "Unable to get identities: %s", err)
	fmt.Printf("%#v\n", identities)
	assert.Truef(t, len(identities.Identities) == 1, "Wrong identities length %d", len(identities.Identities))
}

func TestACLs(t *testing.T) {
	client := createClient(t)
	identities, err := client.GetIdentities([]string{"glenn.tarcea@gmail.com"})
	assert.NoErrorf(t, err, "Unable to get identities: %s", err)
	userGlobusIdentity := identities.Identities[0].ID

	tests := []struct {
		identity   string
		path       string
		shouldFail bool
		deleteACL  bool
		name       string
	}{
		{identity: userGlobusIdentity, path: "/~/globus-staging/", shouldFail: false, deleteACL: false, name: "Add New ACL"},
		{identity: userGlobusIdentity, path: "/~/globus-staging/", shouldFail: false, deleteACL: true, name: "Add Existing ACL"},
		{identity: userGlobusIdentity, path: "/~/globus-staging", shouldFail: true, deleteACL: false, name: "Bad Path"},
	}

	for _, test := range tests {
		rule := EndpointACLRule{
			PrincipalType: "identity",
			EndpointID:    testEndpointID,
			Path:          test.path,
			IdentityID:    test.identity,
			Permissions:   "rw",
		}

		aclRes, err := client.AddEndpointACLRule(rule)
		if !test.shouldFail {
			assert.NoErrorf(t, err, "Unable to set ACL rule: %s - %#v", err, client.GetGlobusErrorResponse())
			if test.deleteACL {
				_, err := client.DeleteEndpointACLRule(testEndpointID, aclRes.AccessID)
				assert.NoErrorf(t, err, "Unable to delete ACL rule: %s - %#v", err, client.GetGlobusErrorResponse())
			}
		} else {
			assert.Errorf(t, err, "Test should have failed")
		}
	}
}

func TestGetTasks(t *testing.T) {
	client := createClient(t)
	lastWeek := time.Now().AddDate(0, 0, -10).Format("2006-01-02")
	fmt.Println("lastWeek", lastWeek)
	tasks, err := client.GetEndpointTaskList(testEndpointID, map[string]string{
		//"filter_completion_time": lastWeek,
		"filter_status": "SUCCEEDED",
	})
	fmt.Println("GetEndpointTaskList err", err)
	fmt.Printf("   tasks: %#v\n", tasks)
	//for _, task := range tasks.Tasks {
	//	transfers, err := client.GetTaskSuccessfulTransfers(task.TaskID, 0)
	//	fmt.Println("  GetTaskSuccessfulTransfers err", err)
	//	fmt.Printf("    transfers: %#v\n", transfers)
	//	transferItem := transfers.Transfers[0]
	//	pieces := strings.Split(transferItem.DestinationPath, "/")
	//	fmt.Println(len(pieces))
	//	fmt.Printf("pieces[0] = '%s'\n", pieces[0])
	//	fmt.Println("id =", pieces[2])
	//}
}

func createClient(t *testing.T) *Client {
	os.Setenv("MC_CONFIDENTIAL_CLIENT_USER", "54de53f4-1eb5-456b-bcd2-68414351ec02")
	os.Setenv("MC_CONFIDENTIAL_CLIENT_PW", "fs08VRoIh0dIpV8ybkDEDXnv3kC6nlFR0TrTNBYtNz4=")
	os.Setenv("MC_CONFIDENTIAL_CLIENT_ENDPOINT", "0b2ada36-bf56-11ed-9614-4b6fcc022e5a")
	globusCCUser := os.Getenv("MC_CONFIDENTIAL_CLIENT_USER")
	globusCCToken := os.Getenv("MC_CONFIDENTIAL_CLIENT_PW")
	testEndpointID = os.Getenv("MC_CONFIDENTIAL_CLIENT_ENDPOINT")

	if globusCCUser != "" && globusCCToken != "" && testEndpointID != "" {
		client, err := CreateConfidentialClient(globusCCUser, globusCCToken)
		assert.NoErrorf(t, err, "Unable to create confidential client: %s", err)
		return client
	} else {
		t.Skipf("One or more of MC_CONFIDENTIAL_CLIENT_USER, MC_CONFIDENTIAL_CLIENT_PW, MC_CONFIDENTIAL_CLIENT_ENDPOINT not set")
		return nil
	}
}
