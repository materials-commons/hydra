package globus

type MockClient struct {
	err           error
	tasks         []TaskList
	transferItems map[string]TransferItems
}

func NewMockClient() *MockClient {
	return &MockClient{transferItems: make(map[string]TransferItems)}
}

func (c *MockClient) SetError(err error) {
	c.err = err
}

func (c *MockClient) SetTasks(tasks []TaskList) {
	c.tasks = tasks
}

func (c *MockClient) SetTransfersForTask(taskID string, t TransferItems) {
	c.transferItems[taskID] = t
}

func (c *MockClient) Authenticate() error {
	return c.err
}

func (c *MockClient) GetEndpointTaskList(endpointID string, filters map[string]string) (TaskList, error) {
	if c.err != nil {
		return TaskList{}, c.err
	}

	return TaskList{}, c.err
}

func (c *MockClient) GetTaskSuccessfulTransfers(taskID string, marker int) (TransferItems, error) {
	return TransferItems{}, c.err
}

func (c *MockClient) GetIdentities(users []string) (Identities, error) { return Identities{}, c.err }

func (c *MockClient) AddEndpointACLRule(rule EndpointACLRule) (AddEndpointACLRuleResult, error) {
	return AddEndpointACLRuleResult{}, c.err
}

func (c *MockClient) DeleteEndpointACLRule(endpointID string, accessID int) (DeleteEndpointACLRuleResult, error) {
	return DeleteEndpointACLRuleResult{}, c.err
}

func (c *MockClient) Err(err error) *MockClient {
	c.err = err
	return c
}
