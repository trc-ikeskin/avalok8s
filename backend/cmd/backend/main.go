package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {

	namespace := parseFlags()

	/*
		filepath.Join() constructs a file path and homeDir() retrieve
		the current user's home directory ~/.kube/config is the default
		location.
	*/
	kubeconfig := filepath.Join(
		homeDir(), ".kube", "config",
	)

	/*
		clientcmd.BuildConfigFromFlags() reads the kubeconfig file to build
		a configuration object (config) for connecting to the Kubernetes API.
		The empty string "" indicates that no custom API server address is provided.
	*/
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(fmt.Errorf("failed to load kubeconfig: %v", err))
	}

	/*
		kubernetes.NewForConfig(config) creates a clientset, which is an object that
		contains methods for interacting with various Kubernetes resources (e.g., pods,
		services, deployments, etc.).
	*/
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(fmt.Errorf("failed to create clientset: %v", err))
	}

	/*
		List
	*/
	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(fmt.Errorf("failed to list pods in namespace '%s': %v", namespace, err))
	}

	fmt.Printf("Pods in namespace '%s':\n", namespace)
	for _, pod := range pods.Items {
		fmt.Printf("%s\n", pod.Name)
	}
}

// -- Helper Functions

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // if you run on Windows
}

func parseFlags() string {

	namespace := flag.String("namespace", "default", "The namespace to list pods from")
	ns := flag.String("n", "default", "Alias for --namespace")
	flag.Parse()

	if *ns != "default" {
		return *ns
	}

	return *namespace
}
