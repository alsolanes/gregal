package config

import "testing"

func TestInitialChatRoleStillRoutesHeavyCodingTask(t *testing.T) {
	c := routerCfg()
	role, why := c.Route("Refactor tots els fitxers del projecte", false, "chat")
	if role != "code" || why == "" {
		t.Fatalf("default chat trapped heavy task: role=%q why=%q", role, why)
	}
}
