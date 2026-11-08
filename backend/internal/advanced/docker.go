package advanced

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var containerPattern = regexp.MustCompile(`^[a-f0-9]{12,64}$`)

type Container struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Image      string `json:"image"`
	State      string `json:"state"`
	Status     string `json:"status"`
	Ports      string `json:"ports"`
	CreatedAt  string `json:"created_at"`
	RunningFor string `json:"running_for"`
}

type DockerImage struct {
	ID         string `json:"id"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Size       string `json:"size"`
	Created    string `json:"created"`
}

type DockerOverview struct {
	Available  bool          `json:"available"`
	Containers []Container   `json:"containers"`
	Images     []DockerImage `json:"images"`
	Truncated  bool          `json:"truncated"`
	Collected  time.Time     `json:"collected_at"`
}

type DockerLogs struct {
	Container string `json:"container"`
	Output    string `json:"output"`
	Truncated bool   `json:"truncated"`
}

func (s *Service) Docker(ctx context.Context, ownerID, serverID string) (*DockerOverview, error) {
	client, _, err := s.servers.Connect(ctx, ownerID, serverID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	psOutput, stderr, err := s.run(ctx, client, `docker ps -a --no-trunc --format '{{json .}}'`)
	if err != nil && psOutput == "" {
		return nil, classify(stderr, err)
	}
	result := &DockerOverview{
		Available:  true,
		Containers: make([]Container, 0, 16),
		Images:     make([]DockerImage, 0, 16),
		Collected:  time.Now().UTC(),
	}
	for _, line := range strings.Split(psOutput, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(result.Containers) >= s.cfg.DockerMaxContainers {
			result.Truncated = true
			break
		}
		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		result.Containers = append(result.Containers, Container{
			ID:         jsonString(raw, "ID"),
			Name:       strings.TrimPrefix(jsonString(raw, "Names"), "/"),
			Image:      jsonString(raw, "Image"),
			State:      jsonString(raw, "State"),
			Status:     jsonString(raw, "Status"),
			Ports:      jsonString(raw, "Ports"),
			CreatedAt:  jsonString(raw, "CreatedAt"),
			RunningFor: jsonString(raw, "RunningFor"),
		})
	}

	imageOutput, _, imageErr := s.run(ctx, client, `docker images --format '{{json .}}'`)
	if imageErr == nil {
		for _, line := range strings.Split(imageOutput, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var raw map[string]any
			if err := json.Unmarshal([]byte(line), &raw); err != nil {
				continue
			}
			if len(result.Images) >= s.cfg.DockerMaxContainers {
				result.Truncated = true
				break
			}
			result.Images = append(result.Images, DockerImage{
				ID: jsonString(raw, "ID"), Repository: jsonString(raw, "Repository"),
				Tag: jsonString(raw, "Tag"), Size: jsonString(raw, "Size"), Created: jsonString(raw, "CreatedSince"),
			})
		}
	}
	return result, nil
}

func (s *Service) DockerAction(ctx context.Context, ownerID, serverID, containerID, action string) error {
	if !containerPattern.MatchString(containerID) {
		return ErrInvalidTarget
	}
	var command string
	switch action {
	case "start", "stop", "restart", "remove":
		command = "docker " + action + " " + containerID
		if action == "remove" {
			command = "docker rm " + containerID
		}
	default:
		return ErrInvalidTarget
	}
	client, _, err := s.servers.Connect(ctx, ownerID, serverID)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	_, stderr, err := s.run(ctx, client, command)
	if err != nil {
		code := "DOCKER_ACTION_FAILED"
		if errors.Is(classify(stderr, err), ErrPermission) {
			code = "DOCKER_PERMISSION_DENIED"
		}
		s.record(ctx, ownerID, serverID, "docker.action", action+" "+containerID, "failure", code)
		return classify(stderr, err)
	}
	s.record(ctx, ownerID, serverID, "docker.action", action+" "+containerID, "success", "")
	return nil
}

func (s *Service) DockerLogs(ctx context.Context, ownerID, serverID, containerID string, tail int) (*DockerLogs, error) {
	if !containerPattern.MatchString(containerID) {
		return nil, ErrInvalidTarget
	}
	if tail < 10 {
		tail = 10
	}
	if tail > 1000 {
		tail = 1000
	}
	client, _, err := s.servers.Connect(ctx, ownerID, serverID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()
	stdout, stderr, err := s.run(ctx, client, fmt.Sprintf("docker logs --tail %d %s", tail, containerID))
	if err != nil && stdout == "" {
		return nil, classify(stderr, err)
	}
	combined := stdout
	if stderr != "" {
		combined += stderr
	}
	return &DockerLogs{
		Container: containerID,
		Output:    combined,
		Truncated: strings.Contains(combined, "[output truncated]"),
	}, nil
}

func (s *Service) DockerInspect(ctx context.Context, ownerID, serverID, containerID string) (map[string]any, error) {
	if !containerPattern.MatchString(containerID) {
		return nil, ErrInvalidTarget
	}
	client, _, err := s.servers.Connect(ctx, ownerID, serverID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()
	output, stderr, err := s.run(ctx, client, "docker inspect "+containerID)
	if err != nil {
		return nil, classify(stderr, err)
	}
	var raw []map[string]any
	if err := json.Unmarshal([]byte(output), &raw); err != nil || len(raw) == 0 {
		return nil, fmt.Errorf("%w: inspect returned invalid JSON", ErrUnavailable)
	}
	// Docker inspect includes Config.Env and Cmd, which commonly contain
	// secrets. Return only identity, state, mounts, ports and labels.
	item := raw[0]
	network, _ := item["NetworkSettings"].(map[string]any)
	return map[string]any{
		"id": item["Id"], "name": strings.TrimPrefix(anyString(item["Name"]), "/"),
		"image": item["Image"], "created": item["Created"], "path": item["Path"],
		"state": item["State"], "mounts": item["Mounts"],
		"network_settings": map[string]any{
			"ports": network["Ports"], "ip_address": network["IPAddress"],
		},
		// Labels are container-controlled and can contain secrets; they are
		// deliberately omitted from the response.
	}, nil
}

func jsonString(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}
