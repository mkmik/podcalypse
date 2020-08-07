// Podcalypse is a simple k8s tool that kills pods at a given constant rate, provided they match a label.
//
// This is useful when you want to check that your system (e.g. your load balancer) is configured to tolerate
// a given level of disruption (usually caused by rolling upgrades).
package main

import (
	"context"
	"flag"
	"math/rand"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/bitnami-labs/flagenv"
	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	"k8s.io/kubectl/pkg/util/podutils"
	controllers "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	label = "mkm.pub/podcalipse"
)

var (
	logger = logf.Log.WithName("podcalipse")
	log    = logger.WithName("main")
)

// flags are flags.
type Flags struct {
	Rate   float64
	Min    int
	DryRun bool
	Count  int
}

func (f *Flags) Bind(fs *flag.FlagSet) {
	if fs == nil {
		fs = flag.CommandLine
	}
	fs.Float64Var(&f.Rate, "rate", 1, "Rate of deletes per second (max).")
	fs.IntVar(&f.Min, "min", 1, "Minimum number of pods to keep alive.")
	fs.BoolVar(&f.DryRun, "dry-run", false, "Dry run.")
	fs.IntVar(&f.Count, "count", 0, "Number of times the pods are slayed. 0 means forever.")
}

// slay kills a pod at random.
func slay(ctx context.Context, c client.Client, dryRun bool, min int) error {
	var pods corev1.PodList
	if err := c.List(ctx, &pods,
		client.MatchingLabels{label: "true"},
		client.MatchingFields{"status.phase": "Running"},
	); err != nil {
		return err
	}
	var items []corev1.Pod
	for _, i := range pods.Items {
		if isPodAvailable(&i) {
			items = append(items, i)
		}
	}
	log.Info("count", "orig", len(pods.Items), "filtered", len(items))

	if len(items) <= min {
		return nil
	}

	sort.Slice(items, func(i, j int) bool { return items[i].CreationTimestamp.Before(&items[j].CreationTimestamp) })

	for _, pod := range pods.Items {
		log.V(2).Info("list", "namespace", pod.Namespace, "name", pod.Name, "available", isPodAvailable(&pod))
	}

	if !dryRun {
		pod := items[0]
		log.Info("deleting pod", "namespace", pod.Namespace, "name", pod.Name)

		if err := c.Delete(ctx, &pod); err != nil {
			return err
		}
	}
	return nil
}

// isPodAvailable returns true if the pod is running and can be killed.
func isPodAvailable(pod *corev1.Pod) bool {
	return podutils.IsPodAvailable(pod, 10, metav1.Now())
}

// mainE is the main function, but which can return an error instead of having to log at every error check.
func mainE(flags Flags) error {
	log.Info("main", "flags", flags)

	config, err := controllers.GetConfig()
	if err != nil {
		return err
	}

	c, err := client.New(config, client.Options{})
	if err != nil {
		return err
	}

	ctx := context.Background()
	lim := rate.NewLimiter(rate.Limit(flags.Rate), 1)

	var wg sync.WaitGroup
	for i := 0; flags.Count == 0 || i < flags.Count; i++ {
		wg.Add(1)

		if err := lim.Wait(ctx); err != nil {
			return err
		}
		go func() {
			if err := slay(ctx, c, flags.DryRun, flags.Min); err != nil {
				log.Error(err, "slaying")
			}
			wg.Done()
		}()
	}
	wg.Wait()
	return nil
}

func main() {
	rand.Seed(time.Now().UnixNano())

	var flags Flags
	flags.Bind(nil)
	klog.InitFlags(nil)
	flagenv.SetFlagsFromEnv("PODCALYPSE", flag.CommandLine)
	flag.Parse()
	logf.SetLogger(klogr.New())

	if err := mainE(flags); err != nil {
		log.Error(err, "main")
		os.Exit(1)
	}
}
