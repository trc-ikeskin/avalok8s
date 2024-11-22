package main

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
)

type RawKubeConfig struct {
	Name                     string           `json:"name"`
	Server                   string           `json:"server"`
	CertificateAuthorityData []byte           `json:"ca-data,omitempty"`
	Command                  string           `json:"command"`
	Args                     []string         `json:"args"`
	Env                      []api.ExecEnvVar `json:"env,omitempty"`
}

// Storage Engine for Clients

type KubeClientStore struct {
	data map[string]*kubernetes.Clientset
}

func NewKubeClientStore() (*KubeClientStore, error) {
	return &KubeClientStore{
		data: make(map[string]*kubernetes.Clientset),
	}, nil
}

func (k *KubeClientStore) Set(key string, value *kubernetes.Clientset) error {
	k.data[key] = value
	return nil
}

func (k *KubeClientStore) Get(key string) (*kubernetes.Clientset, error) {
	if _, ok := k.data[key]; !ok {
		return nil, fmt.Errorf("clientset with key %s not found!", key)
	}

	return k.data[key], nil
}

// Storage Middleware

func KubeClientStoreMiddleware(kcs *KubeClientStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("kubeClientStore", kcs)
		c.Next()
	}
}

// ClientSet Creation Helper

func CreateClientSet(apiServer string, execCommand string, execArgs []string, execEnv []api.ExecEnvVar, caData []byte) (*kubernetes.Clientset, error) {
	// Validate CA data
	if len(caData) > 0 {
		block, _ := pem.Decode(caData)
		if block == nil {
			return nil, fmt.Errorf("invalid CA data: could not parse PEM")
		}
		if _, err := x509.ParseCertificate(block.Bytes); err != nil {
			return nil, fmt.Errorf("invalid CA data: %v", err)
		}
	}

	config := &rest.Config{
		Host: apiServer,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: caData,
		},
		ExecProvider: &api.ExecConfig{
			Command:         execCommand,
			Args:            execArgs,
			Env:             execEnv,
			APIVersion:      "client.authentication.k8s.io/v1beta1",
			InteractiveMode: api.NeverExecInteractiveMode,
		},
	}

	// Build the Clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes Clientset: %v", err)
	}

	return clientset, nil
}

// REST API

func GetNodes(c *gin.Context) {
	id := c.Param("id")

	kcs, exists := c.Get("kubeClientStore")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not get datastore"})
		return
	}
	kubeClientStore := kcs.(*KubeClientStore)

	kubeClient, err := kubeClientStore.Get(id)
	if err != nil {
		panic(fmt.Errorf("failed to get KubeClient: %v", err))
	}

	nodes, err := kubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	c.IndentedJSON(http.StatusOK, &nodes)
}

func PostKubeClient(c *gin.Context) {
	var newRawKubeConfig RawKubeConfig

	if err := c.BindJSON(&newRawKubeConfig); err != nil {
		return
	}

	clientset, err := CreateClientSet(newRawKubeConfig.Server, newRawKubeConfig.Command, newRawKubeConfig.Args, newRawKubeConfig.Env, newRawKubeConfig.CertificateAuthorityData)
	if err != nil {
		panic(fmt.Errorf("failed to create Kubernetes clientset: %v", err))
	}

	kcs, exists := c.Get("kubeClientStore")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not get datastore"})
		return
	}
	kubeClientStore := kcs.(*KubeClientStore)

	kubeClientStore.Set(newRawKubeConfig.Name, clientset)
}

func main() {
	kubeClientStore, err := NewKubeClientStore()
	if err != nil {
		panic(fmt.Errorf("failed to create Store for KubeClients: %v", err))
	}

	router := gin.Default()
	router.Use(KubeClientStoreMiddleware(kubeClientStore))

	router.POST("/clients", PostKubeClient)
	router.GET("/client/:id/nodes", GetNodes)

	router.Run(":3000")
}
