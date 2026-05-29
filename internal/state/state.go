// Package state persists engine dedupe state to a Kubernetes ConfigMap.
package state

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Store struct {
	cs        kubernetes.Interface
	namespace string
	name      string
}

func New(cs kubernetes.Interface, ns, name string) *Store {
	return &Store{cs: cs, namespace: ns, name: name}
}

func (s *Store) Save(ctx context.Context, dedupe map[string]time.Time) error {
	m := make(map[string]string, len(dedupe))
	for k, v := range dedupe {
		m[k] = v.UTC().Format(time.RFC3339)
	}
	blob, err := json.Marshal(m)
	if err != nil {
		return err
	}
	cm, err := s.cs.CoreV1().ConfigMaps(s.namespace).Get(ctx, s.name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = s.cs.CoreV1().ConfigMaps(s.namespace).Create(ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: s.name, Namespace: s.namespace},
			Data:       map[string]string{"dedupe.json": string(blob)},
		}, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	cm.Data["dedupe.json"] = string(blob)
	_, err = s.cs.CoreV1().ConfigMaps(s.namespace).Update(ctx, cm, metav1.UpdateOptions{})
	return err
}

// Clear empties the persisted dedupe ConfigMap by writing "{}" to it. Does
// NOT delete the object so the bundled Role does not need the delete verb
// (Kubernetes RBAC also makes resourceNames-restricted delete awkward).
// Used by /reset-dedupe; the next snapshot loop overwrites with the same
// empty map, so the world converges either way.
func (s *Store) Clear(ctx context.Context) error {
	cm, err := s.cs.CoreV1().ConfigMaps(s.namespace).Get(ctx, s.name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	cm.Data["dedupe.json"] = "{}"
	_, err = s.cs.CoreV1().ConfigMaps(s.namespace).Update(ctx, cm, metav1.UpdateOptions{})
	return err
}

func (s *Store) Load(ctx context.Context) (map[string]time.Time, error) {
	cm, err := s.cs.CoreV1().ConfigMaps(s.namespace).Get(ctx, s.name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	raw, ok := cm.Data["dedupe.json"]
	if !ok {
		return nil, nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, fmt.Errorf("bad state json: %w", err)
	}
	out := make(map[string]time.Time, len(m))
	for k, v := range m {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			continue
		}
		out[k] = t
	}
	return out, nil
}
