/*
Copyright 2017 The Kubernetes Authors.

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

package settings

import (
	"fmt"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/parnurzeal/gorequest"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/ingress-nginx/test/e2e/framework"
)

var _ = framework.IngressNginxDescribe("[Flag] custom HTTP and HTTPS ports", func() {

	host := "forwarded-headers"

	f := framework.NewDefaultFramework("forwarded-port-headers")

	BeforeEach(func() {
		f.NewEchoDeployment()

		f.WaitForNginxServer("_",
			func(server string) bool {
				return strings.Contains(server, "listen 1443")
			})
	})

	Context("with a plain HTTP ingress", func() {
		It("should set X-Forwarded-Port headers accordingly when listening on a non-default HTTP port", func() {

			ing := framework.NewSingleIngress(host, "/", host, f.Namespace, framework.EchoService, 80, nil)
			f.EnsureIngress(ing)

			f.WaitForNginxServer(host,
				func(server string) bool {
					return strings.Contains(server, "server_name forwarded-headers")
				})

			resp, body, errs := gorequest.New().
				Get(f.GetURL(framework.HTTP)).
				Set("Host", host).
				End()

			Expect(errs).Should(BeEmpty())
			Expect(resp.StatusCode).Should(Equal(http.StatusOK))
			Expect(body).Should(ContainSubstring(fmt.Sprintf("x-forwarded-port=%d", 1080)))
		})
	})

	Context("with a TLS enabled ingress", func() {

		It("should set X-Forwarded-Port header to 443", func() {

			ing := framework.NewSingleIngressWithTLS(host, "/", host, []string{host}, f.Namespace, framework.EchoService, 80, nil)
			f.EnsureIngress(ing)

			tlsConfig, err := framework.CreateIngressTLSSecret(f.KubeClientSet,
				ing.Spec.TLS[0].Hosts,
				ing.Spec.TLS[0].SecretName,
				ing.Namespace)
			Expect(err).NotTo(HaveOccurred())

			framework.WaitForTLS(f.GetURL(framework.HTTPS), tlsConfig)

			f.WaitForNginxServer(host,
				func(server string) bool {
					return strings.Contains(server, "server_name forwarded-headers")
				})

			resp, body, errs := gorequest.New().
				Get(f.GetURL(framework.HTTPS)).
				TLSClientConfig(tlsConfig).
				Set("Host", host).
				End()
			Expect(errs).Should(BeEmpty())
			Expect(resp.StatusCode).Should(Equal(http.StatusOK))
			Expect(body).Should(ContainSubstring(fmt.Sprintf("x-forwarded-port=443")))
		})

		Context("when external authentication is configured", func() {

			It("should set the X-Forwarded-Port header to 443", func() {

				f.NewHttpbinDeployment()

				var httpbinIP string

				err := framework.WaitForEndpoints(f.KubeClientSet, framework.DefaultTimeout, framework.HTTPBinService, f.Namespace, 1)
				Expect(err).NotTo(HaveOccurred())

				e, err := f.KubeClientSet.CoreV1().Endpoints(f.Namespace).Get(framework.HTTPBinService, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())

				httpbinIP = e.Subsets[0].Addresses[0].IP

				annotations := map[string]string{
					"nginx.ingress.kubernetes.io/auth-url":    fmt.Sprintf("http://%s/basic-auth/user/password", httpbinIP),
					"nginx.ingress.kubernetes.io/auth-signin": "http://$host/auth/start",
				}

				ing := framework.NewSingleIngressWithTLS(host, "/", host, []string{host}, f.Namespace, framework.EchoService, 80, annotations)

				f.EnsureIngress(ing)
				tlsConfig, err := framework.CreateIngressTLSSecret(f.KubeClientSet,
					ing.Spec.TLS[0].Hosts,
					ing.Spec.TLS[0].SecretName,
					ing.Namespace)
				Expect(err).NotTo(HaveOccurred())
				framework.WaitForTLS(f.GetURL(framework.HTTPS), tlsConfig)
				f.WaitForNginxServer(host,
					func(server string) bool {
						return strings.Contains(server, "server_name forwarded-headers")
					})
				resp, body, errs := gorequest.New().
					Get(f.GetURL(framework.HTTPS)).
					TLSClientConfig(tlsConfig).
					Set("Host", host).
					SetBasicAuth("user", "password").
					End()
				Expect(errs).Should(BeEmpty())
				Expect(resp.StatusCode).Should(Equal(http.StatusOK))
				Expect(body).Should(ContainSubstring(fmt.Sprintf("x-forwarded-port=443")))

			})

		})

	})

})
