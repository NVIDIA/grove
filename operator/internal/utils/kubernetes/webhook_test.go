package kubernetes

import (
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"testing"
)

func TestCreateObjectKeyForCreateWebhooks(t *testing.T) {
	const (
		testObjName         = "test-obj"
		testObjNamespace    = "test-ns"
		testObjGenerateName = "test-gen-obj"
	)
	testCases := []struct {
		description     string
		obj             client.Object
		admissionReq    admission.Request
		expectObjectKey client.ObjectKey
	}{
		{
			description:     "object with name and without namespace",
			obj:             &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: testObjName}},
			admissionReq:    admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Namespace: testObjNamespace}},
			expectObjectKey: client.ObjectKey{Name: testObjName, Namespace: testObjNamespace},
		},
		{
			description:     "object with generateName and without namespace",
			obj:             &corev1.Pod{ObjectMeta: metav1.ObjectMeta{GenerateName: testObjGenerateName}},
			admissionReq:    admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Namespace: testObjNamespace}},
			expectObjectKey: client.ObjectKey{Name: testObjGenerateName, Namespace: testObjNamespace},
		},
		{
			description:     "object with name and namespace",
			obj:             &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: testObjName, Namespace: testObjNamespace}},
			admissionReq:    admission.Request{},
			expectObjectKey: client.ObjectKey{Name: testObjName, Namespace: testObjNamespace},
		},
		{
			description:     "object with generateName and namespace",
			obj:             &corev1.Pod{ObjectMeta: metav1.ObjectMeta{GenerateName: testObjGenerateName, Namespace: testObjNamespace}},
			admissionReq:    admission.Request{},
			expectObjectKey: client.ObjectKey{Name: testObjGenerateName, Namespace: testObjNamespace},
		},
		{
			description:     "non-namespaced object with name",
			obj:             &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: testObjName}},
			admissionReq:    admission.Request{},
			expectObjectKey: client.ObjectKey{Namespace: "", Name: testObjName},
		},
		{
			description:     "non-namespaced object with generateName",
			obj:             &corev1.Node{ObjectMeta: metav1.ObjectMeta{GenerateName: testObjGenerateName}},
			admissionReq:    admission.Request{},
			expectObjectKey: client.ObjectKey{Namespace: "", Name: testObjGenerateName},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			actualObjectKey := CreateObjectKeyForCreateWebhooks(tc.obj, tc.admissionReq)
			assert.Equal(t, tc.expectObjectKey, actualObjectKey)
		})
	}
}
