/*
Copyright 2019 The Jetstack cert-manager contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"testing"

	extv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	coretesting "k8s.io/client-go/testing"

	"github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	testpkg "github.com/jetstack/cert-manager/pkg/controller/test"
	"github.com/jetstack/cert-manager/test/unit/gen"
)

const testAcmeTLSAnnotation = "kubernetes.io/tls-acme"

func strPtr(s string) *string {
	return &s
}

func TestShouldSync(t *testing.T) {
	type testT struct {
		Annotations map[string]string
		ShouldSync  bool
	}
	tests := []testT{
		{
			Annotations: map[string]string{issuerNameAnnotation: ""},
			ShouldSync:  true,
		},
		{
			Annotations: map[string]string{clusterIssuerNameAnnotation: ""},
			ShouldSync:  true,
		},
		{
			Annotations: map[string]string{testAcmeTLSAnnotation: "true"},
			ShouldSync:  true,
		},
		{
			Annotations: map[string]string{testAcmeTLSAnnotation: "false"},
			ShouldSync:  false,
		},
		{
			Annotations: map[string]string{testAcmeTLSAnnotation: ""},
			ShouldSync:  false,
		},
		{
			Annotations: map[string]string{acmeIssuerChallengeTypeAnnotation: ""},
			ShouldSync:  true,
		},
		{
			Annotations: map[string]string{acmeIssuerDNS01ProviderNameAnnotation: ""},
			ShouldSync:  true,
		},
		{
			ShouldSync: false,
		},
	}
	for _, test := range tests {
		shouldSync := shouldSync(buildIngress("", "", test.Annotations), []string{"kubernetes.io/tls-acme"})
		if shouldSync != test.ShouldSync {
			t.Errorf("Expected shouldSync=%v for annotations %#v", test.ShouldSync, test.Annotations)
		}
	}
}

func TestSync(t *testing.T) {
	clusterIssuer := gen.ClusterIssuer("issuer-name")
	acmeIssuer := gen.Issuer("issuer-name",
		gen.SetIssuerACME(v1alpha1.ACMEIssuer{
			HTTP01: &v1alpha1.ACMEIssuerHTTP01Config{},
			DNS01:  &v1alpha1.ACMEIssuerDNS01Config{},
		}))
	acmeClusterIssuer := gen.ClusterIssuer("issuer-name",
		gen.SetIssuerACME(v1alpha1.ACMEIssuer{
			HTTP01: &v1alpha1.ACMEIssuerHTTP01Config{},
			DNS01:  &v1alpha1.ACMEIssuerDNS01Config{},
		}))
	type testT struct {
		Name                string
		Ingress             *extv1beta1.Ingress
		Issuer              v1alpha1.GenericIssuer
		IssuerLister        []runtime.Object
		ClusterIssuerLister []runtime.Object
		CertificateLister   []runtime.Object
		DefaultIssuerName   string
		DefaultIssuerKind   string
		Err                 bool
		ExpectedCreate      []*v1alpha1.Certificate
		ExpectedUpdate      []*v1alpha1.Certificate
	}
	tests := []testT{
		{
			Name:   "return a single HTTP01 Certificate for an ingress with a single valid TLS entry and HTTP01 annotations using edit-in-place",
			Issuer: acmeClusterIssuer,
			Ingress: &extv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-name",
					Namespace: gen.DefaultTestNamespace,
					Annotations: map[string]string{
						clusterIssuerNameAnnotation:       "issuer-name",
						acmeIssuerChallengeTypeAnnotation: "http01",
						editInPlaceAnnotation:             "true",
					},
				},
				Spec: extv1beta1.IngressSpec{
					TLS: []extv1beta1.IngressTLS{
						{
							Hosts:      []string{"example.com", "www.example.com"},
							SecretName: "example-com-tls",
						},
					},
				},
			},
			ClusterIssuerLister: []runtime.Object{acmeClusterIssuer},
			ExpectedCreate: []*v1alpha1.Certificate{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "example-com-tls",
						Namespace:       gen.DefaultTestNamespace,
						OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(buildIngress("ingress-name", gen.DefaultTestNamespace, nil), ingressGVK)},
					},
					Spec: v1alpha1.CertificateSpec{
						DNSNames:   []string{"example.com", "www.example.com"},
						SecretName: "example-com-tls",
						IssuerRef: v1alpha1.ObjectReference{
							Name: "issuer-name",
							Kind: "ClusterIssuer",
						},
						ACME: &v1alpha1.ACMECertificateConfig{
							Config: []v1alpha1.DomainSolverConfig{
								{
									Domains: []string{"example.com", "www.example.com"},
									SolverConfig: v1alpha1.SolverConfig{
										HTTP01: &v1alpha1.HTTP01SolverConfig{
											Ingress: "ingress-name",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Name:   "return a single HTTP01 Certificate for an ingress with a single valid TLS entry and HTTP01 annotations with no ingress class set",
			Issuer: acmeClusterIssuer,
			Ingress: &extv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-name",
					Namespace: gen.DefaultTestNamespace,
					Annotations: map[string]string{
						clusterIssuerNameAnnotation:       "issuer-name",
						acmeIssuerChallengeTypeAnnotation: "http01",
					},
				},
				Spec: extv1beta1.IngressSpec{
					TLS: []extv1beta1.IngressTLS{
						{
							Hosts:      []string{"example.com", "www.example.com"},
							SecretName: "example-com-tls",
						},
					},
				},
			},
			ClusterIssuerLister: []runtime.Object{acmeClusterIssuer},
			ExpectedCreate: []*v1alpha1.Certificate{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "example-com-tls",
						Namespace:       gen.DefaultTestNamespace,
						OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(buildIngress("ingress-name", gen.DefaultTestNamespace, nil), ingressGVK)},
					},
					Spec: v1alpha1.CertificateSpec{
						DNSNames:   []string{"example.com", "www.example.com"},
						SecretName: "example-com-tls",
						IssuerRef: v1alpha1.ObjectReference{
							Name: "issuer-name",
							Kind: "ClusterIssuer",
						},
						ACME: &v1alpha1.ACMECertificateConfig{
							Config: []v1alpha1.DomainSolverConfig{
								{
									Domains: []string{"example.com", "www.example.com"},
									SolverConfig: v1alpha1.SolverConfig{
										HTTP01: &v1alpha1.HTTP01SolverConfig{},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Name:   "return a single HTTP01 Certificate for an ingress with a single valid TLS entry and HTTP01 annotations with a custom ingress class",
			Issuer: acmeClusterIssuer,
			Ingress: &extv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-name",
					Namespace: gen.DefaultTestNamespace,
					Annotations: map[string]string{
						clusterIssuerNameAnnotation:       "issuer-name",
						acmeIssuerChallengeTypeAnnotation: "http01",
						ingressClassAnnotation:            "nginx-ing",
					},
				},
				Spec: extv1beta1.IngressSpec{
					TLS: []extv1beta1.IngressTLS{
						{
							Hosts:      []string{"example.com", "www.example.com"},
							SecretName: "example-com-tls",
						},
					},
				},
			},
			ClusterIssuerLister: []runtime.Object{acmeClusterIssuer},
			ExpectedCreate: []*v1alpha1.Certificate{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "example-com-tls",
						Namespace:       gen.DefaultTestNamespace,
						OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(buildIngress("ingress-name", gen.DefaultTestNamespace, nil), ingressGVK)},
					},
					Spec: v1alpha1.CertificateSpec{
						DNSNames:   []string{"example.com", "www.example.com"},
						SecretName: "example-com-tls",
						IssuerRef: v1alpha1.ObjectReference{
							Name: "issuer-name",
							Kind: "ClusterIssuer",
						},
						ACME: &v1alpha1.ACMECertificateConfig{
							Config: []v1alpha1.DomainSolverConfig{
								{
									Domains: []string{"example.com", "www.example.com"},
									SolverConfig: v1alpha1.SolverConfig{
										HTTP01: &v1alpha1.HTTP01SolverConfig{
											IngressClass: strPtr("nginx-ing"),
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Name:   "return a single HTTP01 Certificate for an ingress with a single valid TLS entry and HTTP01 annotations with a certificate ingress class",
			Issuer: acmeClusterIssuer,
			Ingress: &extv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-name",
					Namespace: gen.DefaultTestNamespace,
					Annotations: map[string]string{
						clusterIssuerNameAnnotation:            "issuer-name",
						acmeIssuerChallengeTypeAnnotation:      "http01",
						acmeIssuerHTTP01IngressClassAnnotation: "cert-ing",
						ingressClassAnnotation:                 "nginx-ing",
					},
				},
				Spec: extv1beta1.IngressSpec{
					TLS: []extv1beta1.IngressTLS{
						{
							Hosts:      []string{"example.com", "www.example.com"},
							SecretName: "example-com-tls",
						},
					},
				},
			},
			ClusterIssuerLister: []runtime.Object{acmeClusterIssuer},
			ExpectedCreate: []*v1alpha1.Certificate{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "example-com-tls",
						Namespace:       gen.DefaultTestNamespace,
						OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(buildIngress("ingress-name", gen.DefaultTestNamespace, nil), ingressGVK)},
					},
					Spec: v1alpha1.CertificateSpec{
						DNSNames:   []string{"example.com", "www.example.com"},
						SecretName: "example-com-tls",
						IssuerRef: v1alpha1.ObjectReference{
							Name: "issuer-name",
							Kind: "ClusterIssuer",
						},
						ACME: &v1alpha1.ACMECertificateConfig{
							Config: []v1alpha1.DomainSolverConfig{
								{
									Domains: []string{"example.com", "www.example.com"},
									SolverConfig: v1alpha1.SolverConfig{
										HTTP01: &v1alpha1.HTTP01SolverConfig{
											IngressClass: strPtr("cert-ing"),
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Name:   "edit-in-place set to false should not trigger editing the ingress in-place",
			Issuer: acmeClusterIssuer,
			Ingress: &extv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-name",
					Namespace: gen.DefaultTestNamespace,
					Annotations: map[string]string{
						clusterIssuerNameAnnotation:       "issuer-name",
						acmeIssuerChallengeTypeAnnotation: "http01",
						ingressClassAnnotation:            "nginx-ing",
						editInPlaceAnnotation:             "false",
					},
				},
				Spec: extv1beta1.IngressSpec{
					TLS: []extv1beta1.IngressTLS{
						{
							Hosts:      []string{"example.com", "www.example.com"},
							SecretName: "example-com-tls",
						},
					},
				},
			},
			ClusterIssuerLister: []runtime.Object{acmeClusterIssuer},
			ExpectedCreate: []*v1alpha1.Certificate{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "example-com-tls",
						Namespace:       gen.DefaultTestNamespace,
						OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(buildIngress("ingress-name", gen.DefaultTestNamespace, nil), ingressGVK)},
					},
					Spec: v1alpha1.CertificateSpec{
						DNSNames:   []string{"example.com", "www.example.com"},
						SecretName: "example-com-tls",
						IssuerRef: v1alpha1.ObjectReference{
							Name: "issuer-name",
							Kind: "ClusterIssuer",
						},
						ACME: &v1alpha1.ACMECertificateConfig{
							Config: []v1alpha1.DomainSolverConfig{
								{
									Domains: []string{"example.com", "www.example.com"},
									SolverConfig: v1alpha1.SolverConfig{
										HTTP01: &v1alpha1.HTTP01SolverConfig{
											IngressClass: strPtr("nginx-ing"),
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Name:   "should error when an ingress specifies dns01 challenge type but no challenge provider",
			Issuer: acmeClusterIssuer,
			Err:    true,
			Ingress: &extv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-name",
					Namespace: gen.DefaultTestNamespace,
					Annotations: map[string]string{
						clusterIssuerNameAnnotation:       "issuer-name",
						acmeIssuerChallengeTypeAnnotation: "dns01",
					},
				},
				Spec: extv1beta1.IngressSpec{
					TLS: []extv1beta1.IngressTLS{
						{
							Hosts:      []string{"example.com", "www.example.com"},
							SecretName: "example-com-tls",
						},
					},
				},
			},
			ClusterIssuerLister: []runtime.Object{acmeClusterIssuer},
		},
		{
			Name:   "should error when an invalid ACME challenge type is specified",
			Issuer: acmeClusterIssuer,
			Err:    true,
			Ingress: &extv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-name",
					Namespace: gen.DefaultTestNamespace,
					Annotations: map[string]string{
						clusterIssuerNameAnnotation:       "issuer-name",
						acmeIssuerChallengeTypeAnnotation: "invalid-challenge-type",
					},
				},
				Spec: extv1beta1.IngressSpec{
					TLS: []extv1beta1.IngressTLS{
						{
							Hosts:      []string{"example.com", "www.example.com"},
							SecretName: "example-com-tls",
						},
					},
				},
			},
			ClusterIssuerLister: []runtime.Object{acmeClusterIssuer},
		},
		{
			Name:   "return a single DNS01 Certificate for an ingress with a single valid TLS entry and DNS01 annotations",
			Issuer: acmeClusterIssuer,
			Err:    true,
			Ingress: &extv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-name",
					Namespace: gen.DefaultTestNamespace,
					Annotations: map[string]string{
						clusterIssuerNameAnnotation:           "issuer-name",
						acmeIssuerChallengeTypeAnnotation:     "dns01",
						acmeIssuerDNS01ProviderNameAnnotation: "fake-dns",
					},
				},
				Spec: extv1beta1.IngressSpec{
					TLS: []extv1beta1.IngressTLS{
						{
							Hosts:      []string{"example.com", "www.example.com"},
							SecretName: "example-com-tls",
						},
					},
				},
			},
			ClusterIssuerLister: []runtime.Object{acmeClusterIssuer},
			ExpectedCreate: []*v1alpha1.Certificate{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "example-com-tls",
						Namespace:       gen.DefaultTestNamespace,
						OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(buildIngress("ingress-name", gen.DefaultTestNamespace, nil), ingressGVK)},
					},
					Spec: v1alpha1.CertificateSpec{
						DNSNames:   []string{"example.com", "www.example.com"},
						SecretName: "example-com-tls",
						IssuerRef: v1alpha1.ObjectReference{
							Name: "issuer-name",
							Kind: "ClusterIssuer",
						},
						ACME: &v1alpha1.ACMECertificateConfig{
							Config: []v1alpha1.DomainSolverConfig{
								{
									Domains: []string{"example.com", "www.example.com"},
									SolverConfig: v1alpha1.SolverConfig{
										DNS01: &v1alpha1.DNS01SolverConfig{
											Provider: "fake-dns",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Name:   "should error when no challenge type is provided",
			Issuer: acmeClusterIssuer,
			Err:    true,
			Ingress: &extv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-name",
					Namespace: gen.DefaultTestNamespace,
					Annotations: map[string]string{
						clusterIssuerNameAnnotation: "issuer-name",
					},
				},
				Spec: extv1beta1.IngressSpec{
					TLS: []extv1beta1.IngressTLS{
						{
							Hosts:      []string{"example.com", "www.example.com"},
							SecretName: "example-com-tls",
						},
					},
				},
			},
			ClusterIssuerLister: []*v1alpha1.ClusterIssuer{acmeClusterIssuer},
		},
		{
			Name:                "should return a basic certificate when no provider specific config is provided",
			Issuer:              clusterIssuer,
			DefaultIssuerName:   "issuer-name",
			DefaultIssuerKind:   "ClusterIssuer",
			ClusterIssuerLister: []runtime.Object{clusterIssuer},
			Ingress: &extv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-name",
					Namespace: gen.DefaultTestNamespace,
				},
				Spec: extv1beta1.IngressSpec{
					TLS: []extv1beta1.IngressTLS{
						{
							Hosts:      []string{"example.com", "www.example.com"},
							SecretName: "example-com-tls",
						},
					},
				},
			},
			ExpectedCreate: []*v1alpha1.Certificate{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "example-com-tls",
						Namespace:       gen.DefaultTestNamespace,
						OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(buildIngress("ingress-name", gen.DefaultTestNamespace, nil), ingressGVK)},
					},
					Spec: v1alpha1.CertificateSpec{
						DNSNames:   []string{"example.com", "www.example.com"},
						SecretName: "example-com-tls",
						IssuerRef: v1alpha1.ObjectReference{
							Name: "issuer-name",
							Kind: "ClusterIssuer",
						},
					},
				},
			},
		},
		{
			Name:         "should return an error when no TLS hosts are specified",
			Issuer:       acmeIssuer,
			IssuerLister: []runtime.Object{acmeIssuer},
			Err:          true,
			Ingress: &extv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-name",
					Namespace: gen.DefaultTestNamespace,
					Annotations: map[string]string{
						issuerNameAnnotation: "issuer-name",
					},
				},
				Spec: extv1beta1.IngressSpec{
					TLS: []extv1beta1.IngressTLS{
						{
							SecretName: "example-com-tls",
						},
					},
				},
			},
		},
		{
			Name:   "should return an error when no TLS secret name is specified",
			Issuer: acmeIssuer,
			Err:    true,
			Ingress: &extv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-name",
					Namespace: gen.DefaultTestNamespace,
					Annotations: map[string]string{
						issuerNameAnnotation: "issuer-name",
					},
				},
				Spec: extv1beta1.IngressSpec{
					TLS: []extv1beta1.IngressTLS{
						{
							Hosts: []string{"example.com"},
						},
					},
				},
			},
			IssuerLister: []runtime.Object{acmeIssuer},
		},
		{
			Name: "should error if the specified issuer is not found",
			Err:  true,
			Ingress: &extv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-name",
					Namespace: gen.DefaultTestNamespace,
					Annotations: map[string]string{
						issuerNameAnnotation: "invalid-issuer-name",
					},
				},
			},
		},
		{
			Name:         "should not return any certificates if a correct Certificate already exists",
			Issuer:       acmeIssuer,
			IssuerLister: []runtime.Object{acmeIssuer},
			Ingress: &extv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-name",
					Namespace: gen.DefaultTestNamespace,
					Annotations: map[string]string{
						issuerNameAnnotation:              "issuer-name",
						acmeIssuerChallengeTypeAnnotation: "http01",
					},
				},
				Spec: extv1beta1.IngressSpec{
					TLS: []extv1beta1.IngressTLS{
						{
							Hosts:      []string{"example.com"},
							SecretName: "existing-crt",
						},
					},
				},
			},
			CertificateLister: []runtime.Object{
				&v1alpha1.Certificate{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "existing-crt",
						Namespace: gen.DefaultTestNamespace,
					},
					Spec: v1alpha1.CertificateSpec{
						DNSNames:   []string{"example.com"},
						SecretName: "existing-crt",
						IssuerRef: v1alpha1.ObjectReference{
							Name: "issuer-name",
							Kind: "Issuer",
						},
						ACME: &v1alpha1.ACMECertificateConfig{
							Config: []v1alpha1.DomainSolverConfig{
								{
									Domains: []string{"example.com"},
									SolverConfig: v1alpha1.SolverConfig{
										HTTP01: &v1alpha1.HTTP01SolverConfig{
											Ingress: "",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Name:         "should update a certificate if an incorrect Certificate exists",
			Issuer:       acmeIssuer,
			IssuerLister: []runtime.Object{acmeIssuer},
			Ingress: &extv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-name",
					Namespace: gen.DefaultTestNamespace,
					Annotations: map[string]string{
						issuerNameAnnotation:              "issuer-name",
						acmeIssuerChallengeTypeAnnotation: "http01",
					},
				},
				Spec: extv1beta1.IngressSpec{
					TLS: []extv1beta1.IngressTLS{
						{
							Hosts:      []string{"example.com"},
							SecretName: "existing-crt",
						},
					},
				},
			},
			CertificateLister: []runtime.Object{buildCertificate("existing-crt", gen.DefaultTestNamespace)},
			ExpectedUpdate: []*v1alpha1.Certificate{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "existing-crt",
						Namespace: gen.DefaultTestNamespace,
					},
					Spec: v1alpha1.CertificateSpec{
						DNSNames:   []string{"example.com"},
						SecretName: "existing-crt",
						IssuerRef: v1alpha1.ObjectReference{
							Name: "issuer-name",
							Kind: "Issuer",
						},
						ACME: &v1alpha1.ACMECertificateConfig{
							Config: []v1alpha1.DomainSolverConfig{
								{
									Domains: []string{"example.com"},
									SolverConfig: v1alpha1.SolverConfig{
										HTTP01: &v1alpha1.HTTP01SolverConfig{
											Ingress: "",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Name:         "should update a certificate's config if an incorrect Certificate exists",
			Issuer:       acmeIssuer,
			IssuerLister: []runtime.Object{acmeIssuer},
			Ingress: &extv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-name",
					Namespace: gen.DefaultTestNamespace,
					Annotations: map[string]string{
						issuerNameAnnotation:              "issuer-name",
						acmeIssuerChallengeTypeAnnotation: "http01",
						ingressClassAnnotation:            "toot-ing",
					},
				},
				Spec: extv1beta1.IngressSpec{
					TLS: []extv1beta1.IngressTLS{
						{
							Hosts:      []string{"example.com"},
							SecretName: "existing-crt",
						},
					},
				},
			},
			CertificateLister: []runtime.Object{
				&v1alpha1.Certificate{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "existing-crt",
						Namespace: gen.DefaultTestNamespace,
					},
					Spec: v1alpha1.CertificateSpec{
						DNSNames:   []string{"example.com"},
						SecretName: "existing-crt",
						IssuerRef: v1alpha1.ObjectReference{
							Name: "issuer-name",
							Kind: "Issuer",
						},
						ACME: &v1alpha1.ACMECertificateConfig{
							Config: []v1alpha1.DomainSolverConfig{
								{
									Domains: []string{"wrong-example.com"},
									SolverConfig: v1alpha1.SolverConfig{
										HTTP01: &v1alpha1.HTTP01SolverConfig{
											Ingress: "wrong-ingress",
										},
									},
								},
							},
						},
					},
				},
			},
			ExpectedUpdate: []*v1alpha1.Certificate{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "existing-crt",
						Namespace: gen.DefaultTestNamespace,
					},
					Spec: v1alpha1.CertificateSpec{
						DNSNames:   []string{"example.com"},
						SecretName: "existing-crt",
						IssuerRef: v1alpha1.ObjectReference{
							Name: "issuer-name",
							Kind: "Issuer",
						},
						ACME: &v1alpha1.ACMECertificateConfig{
							Config: []v1alpha1.DomainSolverConfig{
								{
									Domains: []string{"example.com"},
									SolverConfig: v1alpha1.SolverConfig{
										HTTP01: &v1alpha1.HTTP01SolverConfig{
											Ingress:      "",
											IngressClass: strPtr("toot-ing"),
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Name:         "should update a Certificate correctly if an existing one of a different type exists",
			Issuer:       acmeIssuer,
			IssuerLister: []runtime.Object{acmeIssuer},
			Ingress: &extv1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-name",
					Namespace: gen.DefaultTestNamespace,
					Annotations: map[string]string{
						issuerNameAnnotation:              "issuer-name",
						acmeIssuerChallengeTypeAnnotation: "http01",
						ingressClassAnnotation:            "toot-ing",
					},
				},
				Spec: extv1beta1.IngressSpec{
					TLS: []extv1beta1.IngressTLS{
						{
							Hosts:      []string{"example.com"},
							SecretName: "existing-crt",
						},
					},
				},
			},
			CertificateLister: []runtime.Object{
				&v1alpha1.Certificate{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "existing-crt",
						Namespace: gen.DefaultTestNamespace,
					},
					Spec: v1alpha1.CertificateSpec{
						DNSNames:   []string{"example.com"},
						SecretName: "existing-crt",
						IssuerRef: v1alpha1.ObjectReference{
							Name: "issuer-name",
							Kind: "Issuer",
						},
					},
				},
			},
			ExpectedUpdate: []*v1alpha1.Certificate{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "existing-crt",
						Namespace: gen.DefaultTestNamespace,
					},
					Spec: v1alpha1.CertificateSpec{
						DNSNames:   []string{"example.com"},
						SecretName: "existing-crt",
						IssuerRef: v1alpha1.ObjectReference{
							Name: "issuer-name",
							Kind: "Issuer",
						},
						ACME: &v1alpha1.ACMECertificateConfig{
							Config: []v1alpha1.DomainSolverConfig{
								{
									Domains: []string{"example.com"},
									SolverConfig: v1alpha1.SolverConfig{
										HTTP01: &v1alpha1.HTTP01SolverConfig{
											Ingress:      "",
											IngressClass: strPtr("toot-ing"),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	testFn := func(test testT) func(t *testing.T) {
		return func(t *testing.T) {
			var allCMObjects []runtime.Object
			allCMObjects = append(allCMObjects, test.IssuerLister...)
			allCMObjects = append(allCMObjects, test.ClusterIssuerLister...)
			allCMObjects = append(allCMObjects, test.CertificateLister...)
			var expectedActions []testpkg.Action
			for _, cr := range test.ExpectedCreate {
				expectedActions = append(expectedActions,
					testpkg.NewAction(coretesting.NewCreateAction(
						v1alpha1.SchemeGroupVersion.WithResource("certificates"),
						cr.Namespace,
						cr,
					)),
				)
			}
			for _, cr := range test.ExpectedUpdate {
				expectedActions = append(expectedActions,
					testpkg.NewAction(coretesting.NewUpdateAction(
						v1alpha1.SchemeGroupVersion.WithResource("certificates"),
						cr.Namespace,
						cr,
					)),
				)
			}
			b := &testpkg.Builder{
				T:                  t,
				CertManagerObjects: allCMObjects,
				ExpectedActions:    expectedActions,
			}
			b.Start()
			defer b.Stop()
			c := &Controller{
				Client:              b.Client,
				CMClient:            b.CMClient,
				Recorder:            b.FakeEventRecorder(),
				issuerLister:        b.SharedInformerFactory.Certmanager().V1alpha1().Issuers().Lister(),
				clusterIssuerLister: b.SharedInformerFactory.Certmanager().V1alpha1().ClusterIssuers().Lister(),
				certificateLister:   b.SharedInformerFactory.Certmanager().V1alpha1().Certificates().Lister(),
				defaults: defaults{
					issuerName:                 test.DefaultIssuerName,
					issuerKind:                 test.DefaultIssuerKind,
					autoCertificateAnnotations: []string{testAcmeTLSAnnotation},
				},
				helper: &fakeHelper{issuer: test.Issuer},
			}
			b.Sync()

			err := c.Sync(context.Background(), test.Ingress)
			if err != nil && !test.Err {
				t.Errorf("Expected no error, but got: %s", err)
			}

			if err := b.AllReactorsCalled(); err != nil {
				t.Errorf("Not all expected reactors were called: %v", err)
			}
			if err := b.AllActionsExecuted(); err != nil {
				t.Errorf(err.Error())
			}
		}
	}
	for _, test := range tests {
		t.Run(test.Name, testFn(test))
	}
}

type fakeHelper struct {
	issuer v1alpha1.GenericIssuer
}

func (f *fakeHelper) GetGenericIssuer(ref v1alpha1.ObjectReference, ns string) (v1alpha1.GenericIssuer, error) {
	if f.issuer == nil {
		return nil, fmt.Errorf("no issuer specified on fake helper")
	}
	return f.issuer, nil
}

func TestIssuerForIngress(t *testing.T) {
	type testT struct {
		Ingress      *extv1beta1.Ingress
		DefaultName  string
		DefaultKind  string
		ExpectedName string
		ExpectedKind string
	}
	tests := []testT{
		{
			Ingress: buildIngress("name", "namespace", map[string]string{
				issuerNameAnnotation: "issuer",
			}),
			ExpectedName: "issuer",
			ExpectedKind: "Issuer",
		},
		{
			Ingress: buildIngress("name", "namespace", map[string]string{
				clusterIssuerNameAnnotation: "clusterissuer",
			}),
			ExpectedName: "clusterissuer",
			ExpectedKind: "ClusterIssuer",
		},
		{
			Ingress: buildIngress("name", "namespace", map[string]string{
				testAcmeTLSAnnotation: "true",
			}),
			DefaultName:  "default-name",
			DefaultKind:  "ClusterIssuer",
			ExpectedName: "default-name",
			ExpectedKind: "ClusterIssuer",
		},
		{
			Ingress: buildIngress("name", "namespace", nil),
		},
	}
	for _, test := range tests {
		c := &Controller{
			defaults: defaults{
				issuerKind: test.DefaultKind,
				issuerName: test.DefaultName,
			},
		}
		name, kind := c.issuerForIngress(test.Ingress)
		if name != test.ExpectedName {
			t.Errorf("expected name to be %q but got %q", test.ExpectedName, name)
		}
		if kind != test.ExpectedKind {
			t.Errorf("expected kind to be %q but got %q", test.ExpectedKind, kind)
		}
	}
}

func buildCertificate(name, namespace string) *v1alpha1.Certificate {
	return &v1alpha1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func buildACMEIssuer(name, namespace string) *v1alpha1.Issuer {
	return &v1alpha1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.IssuerSpec{
			IssuerConfig: v1alpha1.IssuerConfig{
				ACME: &v1alpha1.ACMEIssuer{},
			},
		},
	}
}

func buildIngress(name, namespace string, annotations map[string]string) *extv1beta1.Ingress {
	return &extv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
		},
	}
}
