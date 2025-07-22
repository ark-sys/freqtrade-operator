package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NotificationSpec defines the desired state of Notification
type NotificationSpec struct {
	Telegram  *NotificationTelegram  `json:"telegram,omitempty"`
	Webhook   *NotificationWebhook   `json:"webhook,omitempty"`
	APIServer *NotificationAPIServer `json:"apiServer,omitempty"`
}

type NotificationTelegram struct {
	Enabled             bool    `json:"enabled,omitempty"`
	SecretRef           string  `json:"secretRef,omitempty"` // Reference to Secret containing Telegram credentials
	BalanceDustLevel    float64 `json:"balanceDustLevel,omitempty"`
	Reload              *bool   `json:"reload,omitempty"`
	AllowCustomMessages *bool   `json:"allowCustomMessages,omitempty"`
}

type NotificationWebhook struct {
	Enabled             bool   `json:"enabled,omitempty"`
	URL                 string `json:"url,omitempty"`
	Entry               string `json:"entry,omitempty"`
	EntryCancel         string `json:"entryCancel,omitempty"`
	EntryFill           string `json:"entryFill,omitempty"`
	Exit                string `json:"exit,omitempty"`
	ExitCancel          string `json:"exitCancel,omitempty"`
	ExitFill            string `json:"exitFill,omitempty"`
	Status              string `json:"status,omitempty"`
	AllowCustomMessages *bool  `json:"allowCustomMessages,omitempty"`
}

type NotificationAPIServer struct {
	Enabled       bool   `json:"enabled,omitempty"`
	ListenIP      string `json:"listenIpAddress,omitempty"`
	ListenPort    int    `json:"listenPort,omitempty"`
	Verbosity     string `json:"verbosity,omitempty"`
	EnableOpenAPI *bool  `json:"enableOpenapi,omitempty"` // Enable OpenAPI documentation
	SecretRef     string `json:"secretRef,omitempty"`     // Reference to Secret containing API server credentials
}

type NotificationStatus struct {
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

type Notification struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NotificationSpec   `json:"spec,omitempty"`
	Status NotificationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type NotificationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Notification `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Notification{}, &NotificationList{})
}
