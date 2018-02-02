package builder

import (
	"docker-proxy-command/config"
	"docker-proxy-command/helper"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"

	"github.com/drone/envsubst"
)

func BuildCommandFromConfig(commandName string, cfg *config.Configuration) (*exec.Cmd, error) {
	commandDef, err := cfg.FindCommandByName(commandName)
	if err != nil {
		return nil, err
	}

	commandBuilder := NewDockerCommandBuilder()
	cmd, err := buildCommandFromCommandDefinition(commandDef, commandBuilder)
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

func buildCommandFromCommandDefinition(commandDef *config.CommandDefinition, builder *DockerCommandBuilder) (*exec.Cmd, error) {

	var err error

	builder.AddCmdArguments(os.Args[1:])

	err = autoBuildAttachStreams(builder)
	if err != nil {
		return nil, err
	}

	err = autoBuildTerminalContext(builder)
	if err != nil {
		return nil, err
	}

	err = autoBuildInteractiveMode(builder)
	if err != nil {
		return nil, err
	}

	if commandDef.HasPropertyEntryPoint() {
		builder.SetEntryPoint(*commandDef.EntryPoint)
	}

	if commandDef.HasPropertyNetwork() {
		builder.SetNetwork(*commandDef.Network)
	}

	if commandDef.HasPropertyIsInteractive() && *commandDef.IsInteractive {
		builder.AddArgument("-i")
	}
	if commandDef.HasPropertyRemoveContainer() {
		err = buildRemoveContainer(*commandDef.RemoveContainer, builder)
		if err != nil {
			return nil, err
		}
	}

	if commandDef.HasPropertyAddGroups() {
		err = buildGroups(*commandDef.AddGroups, builder)
		if err != nil {
			return nil, err
		}
	}

	if commandDef.HasPropertyImpersonate() {
		err = buildImpersonation(*commandDef.Impersonate, builder)
		if err != nil {
			return nil, err
		}
	}

	if commandDef.HasPropertyImage() {
		err = buildImage(*commandDef.Image, builder)
		if err != nil {
			return nil, err
		}
	}

	if commandDef.HasPropertyVolumes() {
		err = buildVolumes(*commandDef.Volumes, builder)
		if err != nil {
			return nil, err
		}
	}

	if commandDef.HasPropertyEnvVars() {
		err = buildEnvVars(*commandDef.EnvVars, builder)
		if err != nil {
			return nil, err
		}
	}

	return builder.Build(), nil
}

func buildEnvVars(envVars []string, builder *DockerCommandBuilder) error {
	for _, envVar := range envVars {
		envVarValue, err := resolveEnvVar(envVar)
		if err != nil {
			return err
		}
		builder.AddEnvVar(envVarValue)
	}

	return nil
}

func autoBuildInteractiveMode(builder *DockerCommandBuilder) error {
	builder.AddArgument("-i")

	return nil
}

func autoBuildAttachStreams(builder *DockerCommandBuilder) error {
	builder.
		AttachTo("STDIN").
		AttachTo("STDOUT").
		AttachTo("STDERR")

	return nil
}

func autoBuildTerminalContext(builder *DockerCommandBuilder) error {
	if helper.IsTerminalContext() {
		builder.AddArgument("-t")
	}

	return nil
}

func buildRemoveContainer(isContainerRemoved bool, builder *DockerCommandBuilder) error {
	if !isContainerRemoved {
		return nil
	}

	builder.AddArgument("--rm")

	return nil
}

func buildGroups(areGroupsAdded bool, builder *DockerCommandBuilder) error {
	if !areGroupsAdded {
		return nil
	}

	currentUser, err := user.Current()
	if err != nil {
		return err
	}

	groupIds, err := currentUser.GroupIds()
	if err != nil {
		return err
	}

	if len(groupIds) > 0 {
		for _, groupId := range groupIds {
			builder.AdduserGroup(groupId)
		}
	}

	return nil
}

func buildImpersonation(isImpersonated bool, builder *DockerCommandBuilder) error {
	if !isImpersonated {
		return nil
	}

	currentUser, err := user.Current()
	if err != nil {
		return err
	}

	builder.SetContainerUserAndGroup(currentUser.Uid, currentUser.Gid)

	return nil
}

func buildImage(imageName string, builder *DockerCommandBuilder) error {
	builder.SetImageName(imageName)

	return nil
}

func buildVolumes(volumes []string, builder *DockerCommandBuilder) error {
	for _, volume := range volumes {
		volumeParts := strings.Split(volume, ":")
		if len(volumeParts) < 2 || len(volumeParts) > 3 {
			return fmt.Errorf("invalid number of volume parts in '%s'", volume)
		}

		var hostPart, containerPart, options string
		var resolveErr error

		if len(volumeParts) > 0 {
			hostPart, resolveErr = resolveEnvVar(volumeParts[0])
			if resolveErr != nil {
				return resolveErr
			}
		}
		if len(volumeParts) > 1 {
			containerPart, resolveErr = resolveEnvVar(volumeParts[1])
			if resolveErr != nil {
				return resolveErr
			}
		}
		if len(volumeParts) > 2 {
			options, resolveErr = resolveEnvVar(volumeParts[2])
			if resolveErr != nil {
				return resolveErr
			}
		}

		builder.AddVolumePlain(hostPart, containerPart, options)
	}

	return nil
}

func resolveEnvVar(envVarName string) (string, error) {
	return envsubst.Eval(envVarName, func(normalizedEnvVarName string) string {
		if envVar, ok := os.LookupEnv(normalizedEnvVarName); !ok {
			panic(fmt.Sprintf("env var %v is not set!", normalizedEnvVarName))
		} else {
			return envVar
		}
	})
}
