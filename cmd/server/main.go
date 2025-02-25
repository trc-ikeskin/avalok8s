package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Kubernetes client
var clientset *kubernetes.Clientset

// SSE event channel
var eventChannel = make(chan string)

// Mutex to handle cache concurrency
var cacheMutex sync.RWMutex

// Cached data
var nodesCache []gin.H
var podsCache []gin.H

func init() {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	// creates the clientset
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
}

// Watches Kubernetes nodes and pods for changes
func startKubernetesWatcher(ctx context.Context) {
	log.Println("Starting Kubernetes watch...")

	// Watch Nodes
	go watchResource(ctx, "nodes", clientset.CoreV1().Nodes().Watch)

	// Watch Pods
	go watchResource(ctx, "pods", clientset.CoreV1().Pods("").Watch)
}

// Generic function to watch a Kubernetes resource
func watchResource(ctx context.Context, resource string, watchFunc func(context.Context, metav1.ListOptions) (watch.Interface, error)) {
	for {
		select {
		case <-ctx.Done():
			log.Printf("Stopping watch for %s...", resource)
			return
		default:
			watcher, err := watchFunc(ctx, metav1.ListOptions{})
			if err != nil {
				log.Printf("Error watching %s: %v", resource, err)
				time.Sleep(5 * time.Second)
				continue
			}

			for event := range watcher.ResultChan() {
				data, _ := json.Marshal(event.Object)
				message := fmt.Sprintf("event: %s\ndata: %s\n\n", event.Type, string(data))
				eventChannel <- message
				updateData()
			}
		}
	}
}

// Fetches Kubernetes nodes and pods, updating the cache
func updateData() {
	log.Println("Updating Kubernetes data...")

	newNodes := []gin.H{}
	newPods := []gin.H{}

	// Fetch nodes
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Printf("Error fetching nodes: %v", err)
	} else {
		for _, node := range nodes.Items {
			newNodes = append(newNodes, gin.H{
				"name":   node.Name,
				"status": getNodeStatus(node),
			})
		}
	}

	// Fetch pods
	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Printf("Error fetching pods: %v", err)
	} else {
		for _, pod := range pods.Items {
			newPods = append(newPods, gin.H{
				"namespace": pod.Namespace,
				"name":      pod.Name,
				"status":    string(pod.Status.Phase),
			})
		}
	}

	// Update cache
	cacheMutex.Lock()
	nodesCache = newNodes
	podsCache = newPods
	cacheMutex.Unlock()

	log.Println("Kubernetes data updated.")
}

// Get latest nodes
func GetNodes(c *gin.Context) {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()
	c.JSON(http.StatusOK, gin.H{"nodes": nodesCache})
}

// Get latest pods
func GetPods(c *gin.Context) {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()
	c.JSON(http.StatusOK, gin.H{"pods": podsCache})
}

// SSE Streaming Events
func StreamEvents(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	for {
		select {
		case message := <-eventChannel:
			_, err := c.Writer.Write([]byte(message))
			if err != nil {
				log.Println("Client disconnected from SSE")
				return
			}
			c.Writer.Flush()
		case <-c.Request.Context().Done():
			log.Println("SSE client disconnected")
			return
		}
	}
}

// Extracts "Ready" status from a node
func getNodeStatus(node apiv1.Node) string {
	for _, condition := range node.Status.Conditions {
		if condition.Type == "Ready" && condition.Status == "True" {
			return "Ready"
		}
	}
	return "NotReady"
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

	// Start Kubernetes watcher
	go startKubernetesWatcher(ctx)

	// Create Gin router
	router := gin.Default()
	router.GET("/nodes", GetNodes)
	router.GET("/pods", GetPods)
	router.GET("/events", StreamEvents)

	// Start server
	fmt.Println("Start serving API...")
	router.Run()
}
