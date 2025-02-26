package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"reflect"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	envConfig "github.com/trc-ikeskin/avalok8s/internal/config"
)

var serverConfig envConfig.Config

// Kubernetes client
var clientset *kubernetes.Clientset

// Mutex for cache concurrency
var cacheMutex sync.RWMutex

// Cluster cache
type ClusterState struct {
	Nodes []NodeInfo `json:"nodes"`
}

type NodeInfo struct {
	Name string    `json:"name"`
	Pods []PodInfo `json:"pods"`
}

type PodInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	Node      string `json:"node"`
}

// Cached cluster state
var clusterCache ClusterState

// Create event channel for cluster state changes
var clusterStateEventChannel = make(chan string, 10)

func init() {
	// creates the Kubernetes in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	// creates the Kubernetes ClientSet
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	serverConfig, err = envConfig.NewConfig()
	if err != nil {
		panic(err.Error())
	}
}

// Fetch and update cluster state
func getClusterState() []NodeInfo {
	log.Println("Fetching Kubernetes cluster state...")

	var nodesList []NodeInfo
	nodesMap := make(map[string]NodeInfo)

	// Fetch nodes
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Printf("Error fetching nodes: %v", err)
		return nil
	}

	for _, node := range nodes.Items {
		nodesMap[node.Name] = NodeInfo{
			Name: node.Name,
			Pods: []PodInfo{},
		}
	}

	// Fetch pods
	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Printf("Error fetching pods: %v", err)
		return nil
	}

	for _, pod := range pods.Items {
		podInfo := PodInfo{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Status:    string(pod.Status.Phase),
			Node:      pod.Spec.NodeName,
		}

		// Assign pod to the corresponding node
		if node, exists := nodesMap[pod.Spec.NodeName]; exists {
			node.Pods = append(node.Pods, podInfo)
			nodesMap[pod.Spec.NodeName] = node
		}
	}

	// Convert the map to a slice
	for _, node := range nodesMap {
		nodesList = append(nodesList, node)
	}

	// Sort by node names
	sort.Slice(nodesList, func(i, j int) bool {
		return nodesList[i].Name < nodesList[j].Name
	})

	log.Println("Cluster state fetched successfully.")
	return nodesList
}

func refreshClusterCacheAndNotify() {
	newNodes := getClusterState()
	if newNodes == nil {
		return
	}

	// Acquire read lock to compare with current cache
	cacheMutex.RLock()
	isSameState := reflect.DeepEqual(clusterCache.Nodes, newNodes)
	cacheMutex.RUnlock()

	if isSameState {
		log.Println("No changes detected in cluster state. Skipping cache update.")
		return
	}

	// Update cache only if a real change is detected
	cacheMutex.Lock()
	clusterCache.Nodes = newNodes
	cacheMutex.Unlock()

	// Notify SSE stream about the update
	data, _ := json.Marshal(clusterCache)
	message := fmt.Sprintf("event: %s\ndata: %s\n\n", "updated", string(data))

	select {
	case clusterStateEventChannel <- message:
		log.Println("Cluster state updated and sent to stream.")
	default:
		log.Println("Event channel is full, skipping update notification.")
	}
}

// SSE Streaming Events
func StreamClusterState(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	// Immediately send last known cluster state to new clients
	cacheMutex.RLock()
	initialState, _ := json.Marshal(clusterCache)
	cacheMutex.RUnlock()
	fmt.Fprintf(c.Writer, "event: updated\ndata: %s\n\n", string(initialState))
	c.Writer.Flush()

	for {
		select {
		case message := <-clusterStateEventChannel:
			_, err := c.Writer.Write([]byte(message))
			if err != nil {
				log.Printf("There was an error: %v", err)
				return
			}
			c.Writer.Flush()
		case <-c.Request.Context().Done():
			log.Println("SSE client has disconnected")
			return
		}
	}
}

func main() {
	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigs
		log.Println("Received termination signal. Exiting...")
		cancel()
	}()

	// Fetch initial cluster state
	refreshClusterCacheAndNotify()

	// Schedule updates
	log.Printf("Scheduling queries every %d seconds...", int(serverConfig.QueryInterval.Seconds()))
	ticker := *time.NewTicker(serverConfig.QueryInterval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				refreshClusterCacheAndNotify()
			}
		}
	}()

	// Create Gin router
	router := gin.Default()
	router.Use(cors.Default())

	err := router.SetTrustedProxies(nil)
	if err != nil {
		log.Fatal("Error setting trusted proxies: ", err)
	}

	router.GET("/state", StreamClusterState)

	// Start server
	fmt.Println("Starting to serve API...")
	err = router.Run()
	if err != nil {
		log.Fatal("Error starting server: ", err)
	}
}
