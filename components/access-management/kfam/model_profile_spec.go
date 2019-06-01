/*
 * Kubeflow Auth
 *
 * Access Management API.
 *
 * API version: 1.0.0
 * Contact: kubeflow-engineering@google.com
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */

package kfam

type ProfileSpec struct {

	// Only accept kind: user
	Owner *Subject `json:"owner,omitempty"`
}
