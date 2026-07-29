package cli

import (
	"reflect"
	"testing"
)

func TestModelAndProviderCommandsOpenSeparateScreens(t *testing.T) {
	writeModelSetupConfig(t)

	modelChat := NewModelForLanguage("zh-CN")
	modelChat, modelCmd := modelChat.handleSlashMessageSubmit("/model")
	if modelChat.modelSetup == nil || modelChat.providerSetup != nil || modelCmd == nil {
		t.Fatalf("/model state = %#v", modelChat)
	}

	providerChat := NewModelForLanguage("zh-CN")
	providerChat, providerCmd := providerChat.handleSlashMessageSubmit("/provider")
	if providerChat.providerSetup == nil || providerChat.modelSetup != nil || providerCmd == nil {
		t.Fatalf("/provider state = %#v", providerChat)
	}
}

func TestSlashCommandsSeparateModelAndProviderResponsibilities(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	commands := m.slashCommands()

	gotNames := make([]string, 0, len(commands))
	for _, command := range commands {
		gotNames = append(gotNames, command.Name)
	}
	if want := []string{"/model", "/provider", "/effort", "/skills"}; !reflect.DeepEqual(gotNames, want) {

		t.Fatalf("commands = %#v, want %#v", gotNames, want)
	}
	if commands[0].Description != "选择已配置模型" {
		t.Fatalf("/model description = %q", commands[0].Description)
	}
	if commands[1].Description != "配置模型提供商" {
		t.Fatalf("/provider description = %q", commands[1].Description)
	}
	if commands[3].Description != "列出或调用可用 Skill" {
		t.Fatalf("/skills description = %q", commands[3].Description)
	}
}
