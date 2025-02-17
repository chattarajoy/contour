// Copyright Project Contour Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build e2e
// +build e2e

package httpproxy

import (
	. "github.com/onsi/ginkgo/v2"
	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"github.com/projectcontour/contour/test/e2e"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func testPathPrefixRewrite(namespace string) {
	Specify("path prefix rewrite works", func() {
		t := f.T()

		f.Fixtures.Echo.Deploy(namespace, "no-rewrite")
		f.Fixtures.Echo.Deploy(namespace, "prefix-rewrite")
		f.Fixtures.Echo.Deploy(namespace, "prefix-rewrite-to-root")

		p := &contourv1.HTTPProxy{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "prefix-rewrite",
			},
			Spec: contourv1.HTTPProxySpec{
				VirtualHost: &contourv1.VirtualHost{
					Fqdn: "prefixrewrite.projectcontour.io",
				},
				Routes: []contourv1.Route{
					{
						Services: []contourv1.Service{
							{
								Name: "no-rewrite",
								Port: 80,
							},
						},
						Conditions: []contourv1.MatchCondition{
							{
								Prefix: "/",
							},
						},
					},
					{
						Services: []contourv1.Service{
							{
								Name: "prefix-rewrite",
								Port: 80,
							},
						},
						Conditions: []contourv1.MatchCondition{
							{
								Prefix: "/someprefix1",
							},
						},
						PathRewritePolicy: &contourv1.PathRewritePolicy{
							ReplacePrefix: []contourv1.ReplacePrefix{
								{
									Prefix:      "/someprefix1",
									Replacement: "/someotherprefix",
								},
							},
						},
					},
					{
						Services: []contourv1.Service{
							{
								Name: "prefix-rewrite-to-root",
								Port: 80,
							},
						},
						Conditions: []contourv1.MatchCondition{
							{
								Prefix: "/someprefix2",
							},
						},
						PathRewritePolicy: &contourv1.PathRewritePolicy{
							ReplacePrefix: []contourv1.ReplacePrefix{
								{
									Prefix:      "/someprefix2",
									Replacement: "/",
								},
							},
						},
					},
				},
			},
		}
		f.CreateHTTPProxyAndWaitFor(p, e2e.HTTPProxyValid)

		cases := []struct {
			path            string
			expectedService string
			expectedPath    string
		}{
			{path: "/", expectedService: "no-rewrite", expectedPath: "/"},
			{path: "/foo", expectedService: "no-rewrite", expectedPath: "/foo"},
			{path: "/someprefix1", expectedService: "prefix-rewrite", expectedPath: "/someotherprefix"},
			{path: "/someprefix1foobar", expectedService: "prefix-rewrite", expectedPath: "/someotherprefixfoobar"},
			{path: "/someprefix1/segment", expectedService: "prefix-rewrite", expectedPath: "/someotherprefix/segment"},
			{path: "/someprefix2", expectedService: "prefix-rewrite-to-root", expectedPath: "/"},
			{path: "/someprefix2foobar", expectedService: "prefix-rewrite-to-root", expectedPath: "/foobar"},
			{path: "/someprefix2/segment", expectedService: "prefix-rewrite-to-root", expectedPath: "/segment"},
		}

		for _, tc := range cases {
			t.Logf("Querying %q, expecting service %q and path %q", tc.path, tc.expectedService, tc.expectedPath)

			res, ok := f.HTTP.RequestUntil(&e2e.HTTPRequestOpts{
				Host:      p.Spec.VirtualHost.Fqdn,
				Path:      tc.path,
				Condition: e2e.HasStatusCode(200),
			})
			if !assert.Truef(t, ok, "expected 200 response code, got %d", res.StatusCode) {
				continue
			}

			body := f.GetEchoResponseBody(res.Body)
			assert.Equal(t, tc.expectedService, body.Service)
			assert.Equal(t, tc.expectedPath, body.Path)
		}
	})
}
