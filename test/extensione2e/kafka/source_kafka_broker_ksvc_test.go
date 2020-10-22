package knativekafkae2e

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	eventingsourcesv1beta1 "knative.dev/eventing/pkg/apis/sources/v1beta1"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"github.com/openshift-knative/serverless-operator/test"
)

const (
	kafkaBrokerName  = "smoke-test-kafka-broker"
	kafkatriggerName = "smoke-test-trigger"
	cmName           = "smoke-test-br-cm"
	brokerAPIVersion = "eventing.knative.dev/v1beta1"
	brokerKind       = "Broker"
)

var (
	channelTemplateCM = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: cmName,
		},
		Data: map[string]string{
			"channelTemplateSpec": fmt.Sprintf(`
apiVersion: %q
kind: %q`, channelAPIVersion, kafkaChannelKind),
		},
	}

	broker = &eventingv1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kafkaBrokerName,
			Namespace: testNamespace,
		},
		Spec: eventingv1.BrokerSpec{
			Config: &duckv1.KReference{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Name:       cmName,
			},
		},
	}

	trigger = &eventingv1.Trigger{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kafkatriggerName,
			Namespace: testNamespace,
		},
		Spec: eventingv1.TriggerSpec{
			Broker: kafkaBrokerName,
			Subscriber: duckv1.Destination{
				Ref: &duckv1.KReference{
					APIVersion: ksvcAPIVersion,
					Kind:       ksvcKind,
					Name:       helloWorldService,
				},
			},
		},
	}

	brokerps = &eventingsourcesv1beta1.PingSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pingSourceName,
			Namespace: testNamespace,
		},
		Spec: eventingsourcesv1beta1.PingSourceSpec{
			JsonData: helloWorldText,
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					Ref: &duckv1.KReference{
						APIVersion: brokerAPIVersion,
						Kind:       brokerKind,
						Name:       kafkaBrokerName,
					},
				},
			},
		},
	}
)

func TestSourceToKafkaBrokerToKnativeService(t *testing.T) {
	client := test.SetupClusterAdmin(t)
	cleanup := func() {
		test.CleanupAll(t, client)
		client.Clients.Eventing.EventingV1().Brokers(testNamespace).Delete(kafkaBrokerName, &metav1.DeleteOptions{})
		client.Clients.Eventing.SourcesV1beta1().PingSources(testNamespace).Delete(pingSourceName, &metav1.DeleteOptions{})
		client.Clients.Eventing.EventingV1().Triggers(testNamespace).Delete(kafkatriggerName, &metav1.DeleteOptions{})
		client.Clients.Kube.CoreV1().ConfigMaps(testNamespace).Delete(cmName, &metav1.DeleteOptions{})
	}
	test.CleanupOnInterrupt(t, cleanup)
	defer test.CleanupAll(t, client)
	defer cleanup()

	ksvc, err := test.WithServiceReady(client, helloWorldService, testNamespace, image)
	if err != nil {
		t.Fatal("Knative Service not ready", err)
	}

	// Create the configmap
	_, err = client.Clients.Kube.CoreV1().ConfigMaps(testNamespace).Create(channelTemplateCM)
	if err != nil {
		t.Fatal("Unable to create Channel Template ConfigMap: ", err)
	}

	// Create the (kafka backed) broker
	_, err = client.Clients.Eventing.EventingV1().Brokers(testNamespace).Create(broker)
	if err != nil {
		t.Fatal("Unable to create Kafka Backed Broker: ", err)
	}

	// Create the Trigger
	_, err = client.Clients.Eventing.EventingV1().Triggers(testNamespace).Create(trigger)
	if err != nil {
		t.Fatal("Unable to create trigger: ", err)
	}

	// Create the source
	_, err = client.Clients.Eventing.SourcesV1beta1().PingSources(testNamespace).Create(brokerps)
	if err != nil {
		t.Fatal("Unable to create pingsource: ", err)
	}

	// Wait for text in kservice
	waitForRouteServingText(t, client, ksvc.Status.URL.URL(), helloWorldText)

	// Cleanup
	cleanup()
}
