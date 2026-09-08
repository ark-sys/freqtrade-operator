package tradebot

import (
	"sort"
	"testing"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func tradeBotNamed(name string) *freqtradev1alpha1.TradeBot {
	return &freqtradev1alpha1.TradeBot{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "trading"}}
}

func frequi(name, host string, refs ...string) freqtradev1alpha1.FreqUI {
	return freqtradev1alpha1.FreqUI{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "trading"},
		Spec:       freqtradev1alpha1.FreqUISpec{Host: host, TradeBotRefs: refs},
	}
}

func TestCollectCORSHostsForTradeBot(t *testing.T) {
	tests := []struct {
		name         string
		tradeBotName string
		frequis      []freqtradev1alpha1.FreqUI
		want         []string
	}{
		{
			name:         "no FreqUI references this bot",
			tradeBotName: "my-bot",
			frequis:      []freqtradev1alpha1.FreqUI{frequi("ui", "frequi.example.com", "other-bot")},
			want:         nil,
		},
		{
			name:         "bare hostname defaults to https and adds a bot subdomain",
			tradeBotName: "my-bot",
			frequis:      []freqtradev1alpha1.FreqUI{frequi("ui", "frequi.example.com", "my-bot")},
			want:         []string{"https://frequi.example.com", "https://my-bot.frequi.example.com"},
		},
		{
			name:         "explicit https:// scheme is preserved and parsed",
			tradeBotName: "my-bot",
			frequis:      []freqtradev1alpha1.FreqUI{frequi("ui", "https://frequi.example.com", "my-bot")},
			want:         []string{"https://frequi.example.com", "https://my-bot.frequi.example.com"},
		},
		{
			name:         "explicit http:// scheme is preserved",
			tradeBotName: "my-bot",
			frequis:      []freqtradev1alpha1.FreqUI{frequi("ui", "http://frequi.example.com", "my-bot")},
			want:         []string{"http://frequi.example.com", "http://my-bot.frequi.example.com"},
		},
		{
			name:         "localhost host uses http and skips the bot subdomain",
			tradeBotName: "my-bot",
			frequis:      []freqtradev1alpha1.FreqUI{frequi("ui", "localhost:3000", "my-bot")},
			want:         []string{"http://localhost:3000"},
		},
		{
			name:         "empty host falls back to in-cluster and localhost candidates",
			tradeBotName: "my-bot",
			frequis:      []freqtradev1alpha1.FreqUI{frequi("ui", "", "my-bot")},
			want:         []string{"http://localhost:8080", "http://ui.trading.svc.cluster.local"},
		},
		{
			name:         "multiple FreqUIs referencing the same bot are merged and deduped",
			tradeBotName: "my-bot",
			frequis: []freqtradev1alpha1.FreqUI{
				frequi("ui-a", "frequi.example.com", "my-bot"),
				frequi("ui-b", "frequi.example.com", "my-bot"), // same host -> should dedup
				frequi("ui-c", "other.example.com", "my-bot"),
			},
			want: []string{
				"https://frequi.example.com",
				"https://my-bot.frequi.example.com",
				"https://my-bot.other.example.com",
				"https://other.example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tradeBot := tradeBotNamed(tt.tradeBotName)
			frequiList := &freqtradev1alpha1.FreqUIList{Items: tt.frequis}

			got := collectCORSHostsForTradeBot(tradeBot, frequiList)

			sortedWant := append([]string{}, tt.want...)
			sort.Strings(sortedWant)

			if !equalStringSlices(got, sortedWant) {
				t.Errorf("collectCORSHostsForTradeBot() = %v, want %v", got, sortedWant)
			}
		})
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestValidateCORSHosts(t *testing.T) {
	tests := []struct {
		name    string
		hosts   []string
		wantErr bool
	}{
		{name: "empty list is valid", hosts: nil, wantErr: false},
		{
			name:    "https and http hosts are valid",
			hosts:   []string{"https://a.example.com", "http://localhost:8080"},
			wantErr: false,
		},
		{name: "host without a scheme is invalid", hosts: []string{"a.example.com"}, wantErr: true},
		{
			name:    "one bad host among good ones is still invalid",
			hosts:   []string{"https://a.example.com", "ftp://b.example.com"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCORSHosts(tt.hosts)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCORSHosts(%v) error = %v, wantErr %v", tt.hosts, err, tt.wantErr)
			}
		})
	}
}
