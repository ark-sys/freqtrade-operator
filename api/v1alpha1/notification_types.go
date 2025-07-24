package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NotificationSpec defines the desired state of Notification
type NotificationSpec struct {
	Telegram *NotificationTelegram `json:"telegram,omitempty"`
	Webhook  *NotificationWebhook  `json:"webhook,omitempty"`
	Discord  *NotificationDiscord  `json:"discord,omitempty"`
}

type NotificationTelegram struct {
	Enabled             bool     `json:"enabled,omitempty"`
	Token               string   `json:"token,omitempty"`
	SecretRef           string   `json:"secretRef,omitempty"` // Reference to Secret containing Telegram credentials
	BalanceDustLevel    float64  `json:"balance_dust_level,omitempty"`
	Reload              *bool    `json:"reload,omitempty"`
	AllowCustomMessages *bool    `json:"allow_custom_messages,omitempty"`
	ChatID              string   `json:"chat_id,omitempty"`  // Telegram chat ID to send notifications
	TopicID             string   `json:"topic_id,omitempty"` // Topic ID for grouping notifications
	AuthorizedUsers     []string `json:"authorized_users,omitempty"`

	Settings *NotificationTelegramSettings `json:"settings,omitempty"` // Custom settings for Telegram notifications

}

type NotificationTelegramSettings struct {
	Status                  string `json:"status,omitempty"`
	Warning                 string `json:"warning,omitempty"`
	Startup                 string `json:"startup,omitempty"`                   // Startup message
	Entry                   string `json:"entry,omitempty"`                     // Entry message
	EntryFill               string `json:"entry_fill,omitempty"`                // Entry fill message
	EntryCancel             string `json:"entry_cancel,omitempty"`              // Entry cancel message
	Exit                    string `json:"exit,omitempty"`                      // Exit message
	ExitFill                string `json:"exit_fill,omitempty"`                 // Exit fill message
	ExitCancel              string `json:"exit_cancel,omitempty"`               // Exit cancel message
	ProtectionTrigger       string `json:"protection_trigger,omitempty"`        // Protection trigger message
	ProtectionTriggerGlobal string `json:"protection_trigger_global,omitempty"` // Global protection trigger message

}

type NotificationWebhook struct {
	Enabled             bool   `json:"enabled,omitempty"`
	URL                 string `json:"url,omitempty"`
	Entry               string `json:"entry,omitempty"`
	EntryCancel         string `json:"entry_cancel,omitempty"`
	EntryFill           string `json:"entry_fill,omitempty"`
	Exit                string `json:"exit,omitempty"`
	ExitCancel          string `json:"exit_cancel,omitempty"`
	ExitFill            string `json:"exit_fill,omitempty"`
	Status              string `json:"status,omitempty"`
	AllowCustomMessages *bool  `json:"allow_custom_messages,omitempty"`
}

type NotificationDiscord struct {
	Enabled    bool                `json:"enabled,omitempty"`
	WebhookURL string              `json:"webhook_url,omitempty"` // Discord webhook URL, recommended to be set via environment variable
	ExitFill   []map[string]string `json:"exit_fill,omitempty"`   // Exit fill message template
	EntryFill  []map[string]string `json:"entry_fill,omitempty"`  // Entry fill message template

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
