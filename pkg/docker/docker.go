package docker

import (
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
	"github.com/golang/glog"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

const (
	DOCKER_LABEL_NAMESPACE = "traffic.envoy.namespace"
	DOCKER_LABEL_POD       = "traffic.envoy.pod"
	DOCKER_LABEL_PROXY     = "traffic.envoy.proxy"
)

type DockerInstanceInfo struct {
	ID        string
	Namespace string
	Pod       string
	Status    string
	State     string
}

type DockerClient struct {
	client              *dockerclient.Client
	ProxyPort           string
	ControlPlanePort    string
	ControlPlaneService string
	ProxyManagePort     string
	ProxyUID            string
	ProxyImage          string
	ZipkinService       string
	ZipkinPort          string
}

func NewDockerClient() (*DockerClient, error) {
	var dockerClient DockerClient
	envMap := map[string]*string{
		"ENVOY_PROXY_PORT":       &dockerClient.ProxyPort,
		"ENVOY_PROXY_UID":        &dockerClient.ProxyUID,
		"ENVOY_PROXY_IMAGE":      &dockerClient.ProxyImage,
		"CONTROL_PLANE_PORT":     &dockerClient.ControlPlanePort,
		"CONTROL_PLANE_SERVICE":  &dockerClient.ControlPlaneService,
		"ENVOY_PROXY_MANGE_PORT": &dockerClient.ProxyManagePort,
		"ENVOY_ZIPKIN_SERVICE":   &dockerClient.ZipkinService,
		"ENVOY_ZIPKIN_PORT":      &dockerClient.ZipkinPort,
	}
	for env, envPointer := range envMap {
		*envPointer = os.Getenv(env)
		if *envPointer == "" {
			return nil, fmt.Errorf("Missing env %s", env)
		}
	}
	return &dockerClient, dockerClient.init()
}

func NewSimpleDockerClient() (*DockerClient, error) {
	var dockerClient DockerClient
	var err error
	dockerClient.client, err = dockerclient.NewEnvClient()
	if err != nil {
		return nil, err
	}
	return &dockerClient, nil
}

func (dockerClient *DockerClient) init() error {
	var err error
	dockerClient.client, err = dockerclient.NewEnvClient()
	if err != nil {
		return err
	}

	return dockerClient.PullImage(context.Background(), dockerClient.ProxyImage)
}

func (client *DockerClient) ListDockerInstances(name string) ([]*DockerInstanceInfo, error) {
	var result []*DockerInstanceInfo
	args := filters.NewArgs()
	args.Add("label", DOCKER_LABEL_PROXY)
	if name != "" {
		args.Add("name", name)
	}

	containers, err := client.client.ContainerList(context.Background(), types.ContainerListOptions{Filters: args, All: true})
	if err != nil {
		return nil, err
	}

	for _, container := range containers {
		labels := container.Labels
		ns := labels[DOCKER_LABEL_NAMESPACE]
		pod := labels[DOCKER_LABEL_POD]

		if ns != "" && pod != "" {
			dockerInfo := DockerInstanceInfo{
				ID: container.ID, Namespace: ns, Pod: pod,
				Status: container.Status,
				State:  container.State,
			}
			result = append(result, &dockerInfo)
		}
	}

	return result, nil
}
func (client *DockerClient) GetName(podInfo *kubernetes.PodInfo) string {
	return fmt.Sprintf("envoy_%s_%s", podInfo.Name(), podInfo.Namespace())
}
func (client *DockerClient) CreateDockerInstance(podInfo *kubernetes.PodInfo) (string, error) {
	ctx := context.Background()
	var pauseDocker string

	for _, container := range podInfo.Containers {
		containerJson, err := client.client.ContainerInspect(ctx, container)
		if err != nil {
			glog.Errorf("Failed to inspect docker %s for pod %s", container, podInfo.Name())
			continue
		}
		pauseDocker = string(containerJson.HostConfig.NetworkMode)
		if strings.HasPrefix(pauseDocker, "container:") {
			pauseDocker = pauseDocker[10:]
		} else {
			continue
		}
	}

	if pauseDocker == "" {
		return "", fmt.Errorf("could not find pause docker for %s", podInfo.Name())
	}
	if glog.V(2) {
		glog.Infof("target network container %s for pod %s", pauseDocker, podInfo.Name())
	}
	network := container.NetworkMode(fmt.Sprintf("container:%s", pauseDocker))
	var portList string
	for port, _ := range podInfo.GetPortMap() {
		portList += fmt.Sprintf("%d ", port)
	}
	env := []string{
		fmt.Sprintf("MY_POD_IP=%s", podInfo.PodIP),
		fmt.Sprintf("CONTROL_PLANE_PORT=%s", client.ControlPlanePort),
		fmt.Sprintf("CONTROL_PLANE_SERVICE=%s", client.ControlPlaneService),
		fmt.Sprintf("PROXY_MANAGE_PORT=%s", client.ProxyManagePort),
		fmt.Sprintf("PROXY_PORT=%s", client.ProxyPort),
		fmt.Sprintf("PROXY_UID=%s", client.ProxyUID),
		fmt.Sprintf("ZIPKIN_SERVICE=%s", client.ZipkinService),
		fmt.Sprintf("ZIPKIN_PORT=%s", client.ZipkinPort),

		////used for envoy's --service-cluster option
		fmt.Sprintf("SERVICE_CLUSTER=%s.%s", podInfo.Name(), podInfo.Namespace()),
		fmt.Sprintf("NODE_ID=%s.%s", podInfo.Name(), podInfo.Namespace()),

		fmt.Sprintf("INBOUND_PORTS_INCLUDE=%s", portList),
	}

	proxy_config := &container.Config{
		Env: env,
		Labels: map[string]string{
			DOCKER_LABEL_PROXY:     "true",
			DOCKER_LABEL_NAMESPACE: podInfo.Namespace(),
			DOCKER_LABEL_POD:       podInfo.Name(),
		},
		Image:     client.ProxyImage,
		Tty:       false,
		OpenStdin: false,
	}
	host_config := &container.HostConfig{
		NetworkMode: network,
		Privileged:  true,
	}

	resp, err := client.client.ContainerCreate(ctx, proxy_config, host_config, nil, client.GetName(podInfo))
	if err != nil {
		return "", err
	}
	glog.Infof("Create proxy docker %s for pod %s, env=%v, network:%s", resp.ID, podInfo.Name(), env, pauseDocker)
	err = client.client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		glog.Warningf("Removing proxy docker %s for start failure", resp.ID)
		removeErr := client.client.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{})
		if removeErr != nil {
			glog.Errorf("Remove container failed: %s", removeErr.Error())
		}
		return "", err
	}

	return resp.ID, nil
}
func (client *DockerClient) PullImage(ctx context.Context, imageName string) error {
	var option types.ImagePullOptions

	glog.Infof("Pulling Image %s", imageName)

	out, err := client.client.ImagePull(ctx, imageName, option)
	if err != nil {
		glog.Errorf("Pull image failed: %s", err.Error())
		return err
	}

	defer out.Close()
	body, err := ioutil.ReadAll(out)
	if err != nil {
		glog.Errorf("Read image pulling output failed: %s", err.Error())
	} else {
		lines := strings.Split(string(body), "\n")
		linesNum := len(lines)
		if linesNum > 3 {
			lines = lines[linesNum-3 : linesNum]
		}
		glog.Infof("Pulled Image %s", imageName)
		for _, line := range lines {
			glog.Infof("Status: %s", line)
		}
	}
	return nil
}

func (client *DockerClient) GetDockerInstanceLog(dockerId string) (io.ReadCloser, error) {
	ctx := context.Background()
	return client.client.ContainerLogs(ctx, dockerId, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
}
func (client *DockerClient) IsDockerInstanceRunning(dockerId string) bool {
	ctx := context.Background()
	containerJson, err := client.client.ContainerInspect(ctx, dockerId)
	if err != nil {
		glog.Errorf("Inspect container %s failed: %s", dockerId, err.Error())
		return false
	}
	if containerJson.State != nil {
		return containerJson.State.Running
	} else {
		return false
	}
}

func (client *DockerClient) StopDockerInstance(dockerId string, podName string) {
	ctx := context.Background()
	err := client.client.ContainerStop(ctx, dockerId, nil)
	if err != nil {
		glog.Errorf("Stop container %s failed: %s", dockerId, err.Error())
	} else {
		glog.Infof("Stopped container %s for %s", dockerId, podName)
	}
}

func (client *DockerClient) RemoveDockerInstance(dockerId string, podName string) {
	ctx := context.Background()
	err := client.client.ContainerRemove(ctx, dockerId, types.ContainerRemoveOptions{})
	if err != nil {
		glog.Errorf("Remove container %s failed: %s", dockerId, err.Error())
	} else {
		glog.Infof("Removed container %s for %s", dockerId, podName)
	}
}
