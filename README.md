# aws-eks-auth [![Go Reference](https://pkg.go.dev/badge/github.com/bored-engineer/aws-eks-auth.svg)](https://pkg.go.dev/github.com/bored-engineer/aws-eks-auth)
A straight-forward Golang implementation of the [aws-iam-authenticator](https://github.com/kubernetes-sigs/aws-iam-authenticator) (AWS EKS) [token generation algorithm](https://aws.github.io/aws-eks-best-practices/security/docs/iam/#controlling-access-to-eks-clusters).

## Why?
The [aws-iam-authenticator/pkg/token](https://pkg.go.dev/github.com/kubernetes-sigs/aws-iam-authenticator/pkg/token) package makes use of the [AWS Golang v1 SDK](https://github.com/aws/aws-sdk-go) which has entered [maintenance mode](https://aws.amazon.com/blogs/developer/announcing-end-of-support-for-aws-sdk-for-go-v1-on-july-31-2025) as of 7/31/2024 ([issue #736](https://github.com/kubernetes-sigs/aws-iam-authenticator/issues/736)), this library utilizes the [AWS Golang v2 SDK](https://github.com/aws/aws-sdk-go-v2) to generate tokens. 

Additionally, the [aws-iam-authenticator/pkg/token](https://pkg.go.dev/github.com/kubernetes-sigs/aws-iam-authenticator/pkg/token) package does not properly handle short-lived AWS credentials ([issue #590](https://github.com/kubernetes-sigs/aws-iam-authenticator/issues/590)). This requires clients to use less secure authentication methods like static AWS IAM users or avoid any caching of tokens adding unnecessary latency to each Kubernetes request.

## Usage
```go
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	eksauth "github.com/bored-engineer/aws-eks-auth"
	"golang.org/x/oauth2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	// Load a local kubeconfig using the KUBECONFIG environment variable
	config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		log.Fatalf("clientcmd.BuildConfigFromFlags failed: %v", err)
	}

	// Load some AWS credentials from the default credential chain
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("config.LoadDefaultConfig failed: %v", err)
	}

	// Wrap the http.RoundTripper using our EKS authentication token source
	ts := eksauth.NewFromConfig(cfg, "eks-cluster-name")
	config.Wrap(func(base http.RoundTripper) http.RoundTripper {
		return &oauth2.Transport{
			Source: ts,
			Base:   base,
		}
	})

	// Finally create a clientset using the authenticated config
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("kubernetes.NewForConfig failed: %v", err)
	}
}
```
