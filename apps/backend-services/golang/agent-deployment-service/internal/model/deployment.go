package model

type DeploymentTask struct {
    TenantID     string
    AgentID      string
    ContainerImg string
    Action       string // CREATE, UPDATE, DELETE
}
