package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

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
	Nodes map[string]NodeInfo `json:"nodes"`
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
func updateClusterState() {
	log.Println("Fetching Kubernetes cluster state...")

	newState := ClusterState{Nodes: make(map[string]NodeInfo)}

	// Fetch nodes
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Printf("Error fetching nodes: %v", err)
		return
	}

	for _, node := range nodes.Items {
		newState.Nodes[node.Name] = NodeInfo{
			Name: node.Name,
			Pods: []PodInfo{},
		}
	}

	// Fetch pods
	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Printf("Error fetching pods: %v", err)
		return
	}

	for _, pod := range pods.Items {
		podInfo := PodInfo{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Status:    string(pod.Status.Phase),
			Node:      pod.Spec.NodeName,
		}

		// Assign pod to corresponding node
		if node, exists := newState.Nodes[pod.Spec.NodeName]; exists {
			node.Pods = append(node.Pods, podInfo)
			newState.Nodes[pod.Spec.NodeName] = node
		}
	}

	// Convert both the new and old cache to JSON for comparison
	newStateJSON, _ := json.Marshal(newState)
	cacheMutex.RLock()
	oldStateJSON, _ := json.Marshal(clusterCache)
	cacheMutex.RUnlock()

	// If the state hasn't changed, return without sending an event
	if string(newStateJSON) == string(oldStateJSON) {
		log.Println("No changes in cluster state. Skipping update!")
		return
	}

	// Update cache with minimal locking
	newClusterCache := newState
	cacheMutex.Lock()
	clusterCache = newClusterCache
	cacheMutex.Unlock()
	log.Println("Cluster state cache updated.")

	// Notify consumers via event channel
	data, _ := json.Marshal(clusterCache)
	message := fmt.Sprintf("event: %s\ndata: %s\n\n", "updated", string(data))
	select {
	case clusterStateEventChannel <- message:
		log.Println("Cluster state updated and sent to stream.")
	default:
		log.Println("Event channel is full, skipping update.")
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
	updateClusterState()

	// Schedule updates
	log.Printf("Scheduling queries every %d seconds...", serverConfig.QueryInterval)
	ticker := *time.NewTicker(serverConfig.QueryInterval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				updateClusterState()
			}
		}
	}()

	// Create Gin router
	router := gin.Default()
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
