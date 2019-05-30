package cmd

import (
	"bytes"

	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/fatih/color"

	"github.com/spf13/viper"
)

// Pod defines the pod struct
type Pod struct {
	Name       string
	Namespace  string
	Cluster    string
	Containers []string
}

// WishCtlError defines errors of wishctl
type WishCtlError string

// Error implements error interfaces
func (w WishCtlError) Error() string { return string(w) }

// Struct to hold a cluster's information form the config file
type Cluster struct {
	Name        string
	Region      string
	Environment string
	Az          string
	Hidden      bool
}

// Struct used to unmarshal yaml config
type config struct {
	Clusters []Cluster
}

// Unmarshal config filee
func getConf(configpath string) *config {
	viper.SetConfigFile(configpath)
	err := viper.ReadInConfig()

	if err != nil {
		fmt.Printf("%v", err)
	}

	conf := &config{}
	err = viper.Unmarshal(conf)
	if err != nil {
		fmt.Printf("unable to decode into config struct, %v", err)
	}

	return conf
}

// CLUSTERNOTFOUND defines the error of not finding any cluster
const CLUSTERNOTFOUND = WishCtlError("failed to get cluster, no cluster found")

//Gets a lift off all cluster names
func getAllClusters() ([]string, error) {
	result, err := exec.Command("kubectl", "config", "get-contexts", "-o=name").Output()
	if err != nil {
		return nil, err
	}
	clusterList := strings.Split(strings.TrimSpace(string(result)), "\n")
	return clusterList, nil
}

func filterClusters(clusterList []string, clusterMap map[string]Cluster, region, environment, az string) []string {
	clusters := make([]string, 0)
	for _, c := range clusterList {
		if clusterInfo, ok := clusterMap[c]; ok {
			if (!clusterInfo.Hidden) &&
				(region == "" || strings.Trim(region, " \r\n") == strings.Trim(clusterInfo.Region, " \r\n")) &&
				(environment == "" || strings.Trim(environment, " \r\n") == strings.Trim(clusterInfo.Environment, " \r\n")) &&
				(az == "" || strings.Trim(az, " \r\n") == strings.Trim(clusterInfo.Az, " \r\n")) {
				clusters = append(clusters, c)
			}
		} else {
			fmt.Printf("WARNING: the cluster ", c, " is not defined in the configuration",
				" pods in this cluster are not included in the results \n")
		}
	}
	return clusters
}

//Gets a filterned list of clusters given region, environment and AZ
func getFilteredClusters(configpath, region, environment, az string) ([]string, error) {
	clusterList, err := getAllClusters()
	if err != nil {
		return nil, err
	}

	if configpath == "" {
		configpath = os.Getenv("WISHCTL_CONFIG")
	}

	conf := getConf(configpath)
	clusterMap := make(map[string]Cluster)
	for _, c := range conf.Clusters {
		clusterMap[c.Name] = c
	}

	clusters := filterClusters(clusterList, clusterMap, region, environment, az)

	return clusters, err
}

// PodList defines the list of Pods
type PodList struct {
	ClusterName string
	Pods        []string
}

// String format the PodList struct
func (p PodList) String() string {

	var b bytes.Buffer
	b.WriteString(fmt.Sprintf("Cluster - %s:\n", p.ClusterName))

	if p.Pods == nil {
		b.WriteString("\tnil\n")
		return b.String()
	}

	for _, n := range p.Pods {
		b.WriteString(fmt.Sprintf("\t%s\n", n))
	}

	return b.String()

}

func findPods(pod, namespace, configpath, region, environment, az string) ([]Pod, error) {
	clusters, err := getFilteredClusters(environment, region, environment, az)
	if err != nil {
		return nil, err
	}
	if len(clusters) == 0 {
		return nil, CLUSTERNOTFOUND
	}

	resultChan := make(chan []Pod)

	for _, c := range clusters {
		go findPodsInCluster(pod, c, namespace, resultChan)
	}

	var res []Pod
	for range clusters {
		response := <-resultChan
		if response == nil {
			continue
		}
		res = append(res, response...)
	}

	return res, nil
}

func findPodsInCluster(pod, cluster, namespace string, resultChan chan []Pod) {

	var command *exec.Cmd

	if namespace == "" {
		command = exec.Command("kubectl", "get", "pods",
			"--all-namespaces", "--no-headers", "--context", cluster,
			"-o=custom-columns=NAME:.metadata.name,NAMESPACE:.metadata.namespace,CONTAINERS:.spec.containers[*].name")
	} else {
		command = exec.Command("kubectl", "get", "pods",
			"-n", namespace, "--no-headers", "--context", cluster,
			"-o=custom-columns=NAME:.metadata.name,NAMESPACE:.metadata.namespace,CONTAINERS:.spec.containers[*].name")
	}

	if viper.GetBool("verbose") {
		prettyPrintCmd(command)
	}

	result, err := command.Output()
	if err != nil || string(result) == "" {
		resultChan <- nil
		return
	}

	podList := strings.Split(strings.TrimSpace(string(result)), "\n")
	if len(podList) < 1 {
		resultChan <- nil
		return
	}

	var tmpPodList []Pod
	for _, p := range podList {
		pList := strings.Fields(p)
		name := pList[0]
		namespace := pList[1]
		containers := strings.Split(pList[2], ",")
		if strings.Contains(name, pod) {
			tmpPodList = append(tmpPodList, Pod{name, namespace, cluster, containers})
		}
	}
	resultChan <- tmpPodList
}

// selector prompts a user interface for choosing from multiple options
func selector(options []string) int {
	reader := bufio.NewReader(os.Stdin)
	for i, o := range options {
		fmt.Printf("%d:\t%s\n", i+1, o)
	}
	fmt.Printf("Select a pod/container (1-%d): ", len(options))
	input, _ := reader.ReadString('\n')
	num, err := strconv.Atoi(strings.TrimSpace(input))
	for err != nil || num < 1 || num > len(options) {
		fmt.Printf("Please enter a valid number (1-%d): ", len(options))
		input, _ = reader.ReadString('\n')
		num, err = strconv.Atoi(strings.TrimSpace(input))
	}
	return num - 1
}

func prettyPrintCmd(command *exec.Cmd) {
	var fmtCmd = "Running: "
	for _, a := range command.Args {
		fmtCmd += a + " "
	}
	color.Green(fmtCmd)
}
