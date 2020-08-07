package main

import (
	"context"
	"flag"
	"math/rand"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	controllers "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	annotation = "mkm.pub/podcalipse"
)

var (
	logger = logf.Log.WithName("podcalipse")
	log    = logger.WithName("main")
)

func slay(ctx context.Context, c client.Client) error {
	var pods corev1.PodList
	if err := c.List(ctx, &pods,
		client.MatchingLabels{annotation: "true"},
		client.MatchingFields{"status.phase": "Running"},
	); err != nil {
		return err
	}
	log.Info("list", "count", len(pods.Items))
	if len(pods.Items) == 0 {
		return nil
	}
	rand.Shuffle(len(pods.Items), func(i, j int) { pods.Items[i], pods.Items[j] = pods.Items[j], pods.Items[i] })
	pod := pods.Items[0]
	log.Info("deleting pod", "namespace", pod.Namespace, "name", pod.Name)

	if err := c.Delete(ctx, &pod); err != nil {
		return err
	}
	return nil
}

func mainE() error {
	config, err := controllers.GetConfig()
	if err != nil {
		return err
	}

	c, err := client.New(config, client.Options{})
	if err != nil {
		return err
	}

	ctx := context.Background()

	if err := slay(ctx, c); err != nil {
		return err
	}
	return nil
}

func main() {
	rand.Seed(time.Now().UnixNano())
	klog.InitFlags(nil)
	flag.Parse()
	logf.SetLogger(klogr.New())

	if err := mainE(); err != nil {
		log.Error(err, "main")
		os.Exit(1)
	}
}
