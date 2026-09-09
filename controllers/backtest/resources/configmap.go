// controllers/backtest/resources/configmap.go
package resources

import (
	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildStrategyConfigMap creates the ConfigMap holding this run's strategy script.
func BuildStrategyConfigMap(backtest freqtradev1beta1.Backtest, strategyName, script string) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: backtest.Name + "-strategy", Namespace: backtest.Namespace},
		Data:       map[string]string{strategyName + ".py": script},
	}
}
