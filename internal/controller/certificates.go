package controller

import (
	"bytes"
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"time"

	"github.com/andreasgerstmayr/tempo-helm-operator/api/v1alpha1"
	"github.com/openshift/library-go/pkg/crypto"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/user"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var defaultUserInfo = &user.DefaultInfo{Name: "system:tempostacks", Groups: []string{"system:logging"}}

func createCerts(ctx context.Context, k8sclient client.Client, tempo v1alpha1.TempoMicroservices) ([]client.Object, error) {
	manifests := []client.Object{}

	caSecret, err := createCA(ctx, k8sclient, tempo.GetNamespace(), fmt.Sprintf("%s-tempo-ca-cert", tempo.GetName()))
	if err != nil {
		return nil, err
	}
	manifests = append(manifests, caSecret)

	ca, err := crypto.GetCAFromBytes(caSecret.Data[corev1.TLSCertKey], caSecret.Data[corev1.TLSPrivateKeyKey])
	if err != nil {
		return nil, err
	}

	caCertBytes := caSecret.Data[corev1.TLSCertKey]
	for _, component := range []string{"compactor", "distributor", "ingester", "querier", "query-frontend", "observatorium"} {
		name := fmt.Sprintf("%s-tempo-%s-certs", tempo.GetName(), component)
		hostnames := []string{fmt.Sprintf("%s-tempo-%s", tempo.GetName(), component)}
		componentSecret, err := createServerCert(ctx, k8sclient, tempo.GetNamespace(), name, ca, caCertBytes, defaultUserInfo, hostnames)
		if err != nil {
			return nil, err
		}

		manifests = append(manifests, componentSecret)
	}
	return manifests, nil
}

func createCA(ctx context.Context, k8sclient client.Client, namespace, name string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := k8sclient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, secret)
	if err != nil {
		if apierrors.IsNotFound(err) {
			secret = &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Data: map[string][]byte{},
			}
		} else {
			return nil, err
		}
	}

	_, ok := secret.Data[corev1.TLSCertKey]
	expired := false // TODO: check if cert is expired

	if !ok || expired {
		fmt.Printf("CA cert expired\n")
		caCfg, err := crypto.MakeSelfSignedCAConfigForDuration("operator", 10*time.Hour)
		if err != nil {
			return nil, err
		}

		certBytes := &bytes.Buffer{}
		keyBytes := &bytes.Buffer{}
		err = caCfg.WriteCertConfig(certBytes, keyBytes)
		if err != nil {
			return nil, err
		}

		secret.Data[corev1.TLSCertKey] = certBytes.Bytes()
		secret.Data[corev1.TLSPrivateKeyKey] = keyBytes.Bytes()
	} else {
		fmt.Printf("CA cert not expired\n")
	}

	return secret, nil
}

func createServerCert(ctx context.Context, k8sclient client.Client, namespace, name string, ca *crypto.CA, caCertBytes []byte, user user.Info, hostnames []string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := k8sclient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, secret)
	if err != nil {
		if apierrors.IsNotFound(err) {
			secret = &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Data: map[string][]byte{},
			}
		} else {
			return nil, err
		}
	}

	_, ok := secret.Data[corev1.TLSCertKey]
	expired := false // TODO: check if cert is expired

	if !ok || expired {
		fmt.Printf("%s cert expired\n", hostnames[0])

		addClientAuthUsage := func(cert *x509.Certificate) error {
			cert.ExtKeyUsage = append(cert.ExtKeyUsage, x509.ExtKeyUsageClientAuth)
			return nil
		}

		addSubject := func(cert *x509.Certificate) error {
			cert.Subject = pkix.Name{
				CommonName:   user.GetName(),
				SerialNumber: user.GetUID(),
				Organization: user.GetGroups(),
			}
			return nil
		}

		tlsCfg, err := ca.MakeServerCertForDuration(sets.NewString(hostnames...), 10*time.Hour, addClientAuthUsage, addSubject)
		if err != nil {
			return nil, err
		}

		certBytes := &bytes.Buffer{}
		keyBytes := &bytes.Buffer{}
		err = tlsCfg.WriteCertConfig(certBytes, keyBytes)
		if err != nil {
			return nil, err
		}

		secret.Data[corev1.TLSCertKey] = certBytes.Bytes()
		secret.Data[corev1.TLSPrivateKeyKey] = keyBytes.Bytes()
		secret.Data["ca.crt"] = caCertBytes
	} else {
		fmt.Printf("%s cert not expired\n", hostnames[0])
	}

	return secret, nil
}
