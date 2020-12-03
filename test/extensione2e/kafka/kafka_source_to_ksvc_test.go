package knativekafkae2e

import (
	"context"
	"fmt"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/pkg/utils"

	kafkabindingv1beta1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/bindings/v1beta1"
	kafkasourcev1beta1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/sources/v1beta1"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"github.com/openshift-knative/serverless-operator/test"
	"github.com/openshift-knative/serverless-operator/test/servinge2e"
)

const (
	kafkaSourceName     = "smoke-ks"
	kafkaTopicName      = "smoke-topic"
	kafkaConsumerGroup  = "smoke-cg"
	testNamespace       = "serverless-tests"
	image               = "gcr.io/knative-samples/helloworld-go"
	helloWorldService   = "helloworld-go"
	ksvcAPIVersion      = "serving.knative.dev/v1"
	ksvcKind            = "Service"
	kafkaTopicKind      = "KafkaTopic"
	kafkaAPIVersion     = "kafka.strimzi.io/v1beta1"
	clusterName         = "my-cluster" // there should be a way to get this from test setup
	strimziClusterLabel = "strimzi.io/cluster"
	cronJobName         = "smoke-cronjob"
)

var (
	baseURI              = "-kafka-bootstrap.kafka:"
	plainBootstrapServer = clusterName + baseURI + "9092"
	tlsBootstrapServer   = clusterName + baseURI + "9093"
	saslBootstrapServer  = clusterName + baseURI + "9094"
	tlsSecret            = "my-tls-secret"
	saslSecret           = "my-sasl-secret"
	kafkaGVR             = schema.GroupVersionResource{Group: "kafka.strimzi.io", Version: "v1beta1", Resource: "kafkatopics"}
)

func createCronJobObj(name, topic, server string) *batchv1beta1.CronJob {
	return &batchv1beta1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
		},
		Spec: batchv1beta1.CronJobSpec{
			Schedule: "* * * * *",
			JobTemplate: batchv1beta1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:    "kafka-message-test",
									Image:   "strimzi/kafka:0.16.2-kafka-2.4.0",
									Command: []string{"sh", "-c", fmt.Sprintf(`echo "%s" | bin/kafka-console-producer.sh --broker-list %s --topic %s`, helloWorldText, server, topic)},
								},
							},
							RestartPolicy: corev1.RestartPolicyOnFailure,
						},
					},
				},
			},
		},
	}
}

func createKafkaSourceObj(sourceName, sinkName, topicName string, auth kafkabindingv1beta1.KafkaAuthSpec) kafkasourcev1beta1.KafkaSource {
	return kafkasourcev1beta1.KafkaSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sourceName,
			Namespace: testNamespace,
		},
		Spec: kafkasourcev1beta1.KafkaSourceSpec{
			KafkaAuthSpec: auth,
			Topics:        []string{topicName},
			ConsumerGroup: kafkaConsumerGroup,
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					Ref: &duckv1.KReference{
						APIVersion: ksvcAPIVersion,
						Kind:       ksvcKind,
						Name:       sinkName,
					},
				},
			},
		},
	}
}
func createKafkaTopicObj(topicName string) unstructured.Unstructured {
	// We use unstructured to avoid having a hard dep on any specific kafka implementation
	return unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": kafkaAPIVersion,
			"kind":       kafkaTopicKind,
			"metadata": map[string]interface{}{
				"name":      topicName,
				"namespace": testNamespace,
				"labels": map[string]interface{}{
					strimziClusterLabel: clusterName,
				},
			},
			//Taken from https://github.com/strimzi/strimzi-kafka-operator/blob/0.19.0/examples/topic/kafka-topic.yaml
			"spec": map[string]interface{}{
				"partitions": 1,
				"replicas":   1,
			},
		},
	}

}

func TestKafkaSourceToKnativeService(t *testing.T) {
	client := test.SetupClusterAdmin(t)
	cleanup := func() {
		test.CleanupAll(t, client)
		client.Clients.Dynamic.Resource(kafkaGVR).Namespace(testNamespace).Delete(context.Background(), kafkaTopicName+"-plain", metav1.DeleteOptions{})
		client.Clients.Dynamic.Resource(kafkaGVR).Namespace(testNamespace).Delete(context.Background(), kafkaTopicName+"-tls", metav1.DeleteOptions{})
		client.Clients.Dynamic.Resource(kafkaGVR).Namespace(testNamespace).Delete(context.Background(), kafkaTopicName+"-sasl", metav1.DeleteOptions{})
		client.Clients.KafkaSource.SourcesV1beta1().KafkaSources(testNamespace).Delete(context.Background(), kafkaSourceName+"-plain", metav1.DeleteOptions{})
		client.Clients.KafkaSource.SourcesV1beta1().KafkaSources(testNamespace).Delete(context.Background(), kafkaSourceName+"-tls", metav1.DeleteOptions{})
		client.Clients.KafkaSource.SourcesV1beta1().KafkaSources(testNamespace).Delete(context.Background(), kafkaSourceName+"-sasl", metav1.DeleteOptions{})
		client.Clients.Kube.BatchV1beta1().CronJobs(testNamespace).Delete(context.Background(), cronJobName+"-plain", metav1.DeleteOptions{})
		client.Clients.Kube.BatchV1beta1().CronJobs(testNamespace).Delete(context.Background(), cronJobName+"-tls", metav1.DeleteOptions{})
		client.Clients.Kube.BatchV1beta1().CronJobs(testNamespace).Delete(context.Background(), cronJobName+"-sasl", metav1.DeleteOptions{})
		client.Clients.Kube.CoreV1().Secrets(testNamespace).Delete(context.Background(), tlsSecret, metav1.DeleteOptions{})
		client.Clients.Kube.CoreV1().Secrets(testNamespace).Delete(context.Background(), saslSecret, metav1.DeleteOptions{})
	}
	test.CleanupOnInterrupt(t, cleanup)
	defer cleanup()

	// Get Secret Name -> AuthSecretName
	_, err := utils.CopySecret(client.Clients.Kube.CoreV1(), "default", tlsSecret, testNamespace, "default")
	if err != nil {
		t.Fatalf("Could not copy Secret: %s to test namespace: %s", tlsSecret, testNamespace)
	}

	_, err = utils.CopySecret(client.Clients.Kube.CoreV1(), "default", saslSecret, testNamespace, "default")
	if err != nil {
		t.Fatalf("Could not copy Secret: %s to test namespace: %s", saslSecret, testNamespace)
	}

	tests := map[string]kafkabindingv1beta1.KafkaAuthSpec{
		"plain": {
			BootstrapServers: []string{plainBootstrapServer},
		},
		"tls": {
			BootstrapServers: []string{tlsBootstrapServer},
			Net: kafkabindingv1beta1.KafkaNetSpec{
				TLS: kafkabindingv1beta1.KafkaTLSSpec{
					Enable: true,
					Cert: kafkabindingv1beta1.SecretValueFromSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: tlsSecret,
							},
							Key: "user.crt",
						},
					},
					Key: kafkabindingv1beta1.SecretValueFromSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: tlsSecret,
							},
							Key: "user.key",
						},
					},
					CACert: kafkabindingv1beta1.SecretValueFromSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: tlsSecret,
							},
							Key: "ca.crt",
						},
					},
				},
			},
		},
		"sasl": {
			BootstrapServers: []string{saslBootstrapServer},
			Net: kafkabindingv1beta1.KafkaNetSpec{
				TLS: kafkabindingv1beta1.KafkaTLSSpec{
					Enable: true,
					CACert: kafkabindingv1beta1.SecretValueFromSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: tlsSecret,
							},
							Key: "ca.crt",
						},
					},
				},
				SASL: kafkabindingv1beta1.KafkaSASLSpec{
					Enable: true,
					User: kafkabindingv1beta1.SecretValueFromSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: saslSecret,
							},
							Key: "user",
						},
					},
					Password: kafkabindingv1beta1.SecretValueFromSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: saslSecret,
							},
							Key: "password",
						},
					},
					Type: kafkabindingv1beta1.SecretValueFromSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: saslSecret,
							},
							Key: "saslType",
						},
					},
				},
			},
		},
	}

	for name, tc := range tests {
		name := name
		// Setup a knative service
		ksvc, err := test.WithServiceReady(client, helloWorldService+"-"+name, testNamespace, image)
		if err != nil {
			t.Fatalf("Knative Service(%s) not ready: %v", ksvc.GetName(), err)
		}

		// Create kafkatopic
		kafkaTopicObj := createKafkaTopicObj(kafkaTopicName + "-" + name)
		_, err = client.Clients.Dynamic.Resource(kafkaGVR).Namespace(testNamespace).Create(context.Background(), &kafkaTopicObj, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Unable to create KafkaTopic(%s): %v", kafkaTopicObj.GetName(), err)
		}

		// create kafka source
		kafkaSource := createKafkaSourceObj(kafkaSourceName+"-"+name, helloWorldService+"-"+name, kafkaTopicName+"-"+name, tc)
		_, err = client.Clients.KafkaSource.SourcesV1beta1().KafkaSources(testNamespace).Create(context.Background(), &kafkaSource, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Unable to create kafkaSource(%s): %v", kafkaSource.GetName(), err)
		}

		// send event to kafka topic
		cj := createCronJobObj(cronJobName+"-"+name, kafkaTopicName+"-"+name, kafkaSource.Spec.BootstrapServers[0])
		_, err = client.Clients.Kube.BatchV1beta1().CronJobs(testNamespace).Create(context.Background(), cj, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Unable to create batch cronjob(%s): %v", cj.GetName(), err)
		}

		servinge2e.WaitForRouteServingText(t, client, ksvc.Status.URL.URL(), helloWorldText)
	}
	// cleanup if everything ends smoothly
	cleanup()
}
