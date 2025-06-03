/*
 @Version : 1.0
 @Author  : steven.wong
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2025/06/03 14:15:43
 Desc     :
*/

package service

import (
	"context"
	"fmt"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type Kube struct {
	ctx              context.Context
	kubeClientSet    *kubernetes.Clientset
	dynamicClientSet *dynamic.DynamicClient
}

var (
	kube *Kube
)

func NewKube(ctx context.Context, master, kubeConfig string) *Kube {
	restConfig, err := clientcmd.BuildConfigFromFlags(master, kubeConfig)
	if err != nil {
		panic(fmt.Errorf("error building kubeconfig: %s", err.Error()))
	}
	kubeClientSet := kubernetes.NewForConfigOrDie(restConfig)
	dynamicClientSet := dynamic.NewForConfigOrDie(restConfig)
	kube = &Kube{ctx: ctx, kubeClientSet: kubeClientSet, dynamicClientSet: dynamicClientSet}
	return kube
}

func GetKube() *Kube {
	return kube
}
