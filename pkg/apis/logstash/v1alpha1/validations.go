// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package v1alpha1

import (
	"fmt"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/validation/field"

	commonv1 "github.com/elastic/cloud-on-k8s/v2/pkg/apis/common/v1"
	"github.com/elastic/cloud-on-k8s/v2/pkg/controller/common/version"
)

const (
	pvcImmutableMsg = "Volume claim templates cannot be modified"
)

var (
	defaultChecks = []func(*Logstash) field.ErrorList{
		checkNoUnknownFields,
		checkNameLength,
		checkSupportedVersion,
		checkSingleConfigSource,
		checkESRefsNamed,
		checkAssociations,
		checkSinglePipelineSource,
	}

	updateChecks = []func(old, curr *Logstash) field.ErrorList{
		checkNoDowngrade,
		checkPVCchanges,
	}
)

func checkNoUnknownFields(l *Logstash) field.ErrorList {
	return commonv1.NoUnknownFields(l, l.ObjectMeta)
}

func checkNameLength(l *Logstash) field.ErrorList {
	return commonv1.CheckNameLength(l)
}

func checkSupportedVersion(l *Logstash) field.ErrorList {
	return commonv1.CheckSupportedStackVersion(l.Spec.Version, version.SupportedLogstashVersions)
}

func checkNoDowngrade(prev, curr *Logstash) field.ErrorList {
	if commonv1.IsConfiguredToAllowDowngrades(curr) {
		return nil
	}
	return commonv1.CheckNoDowngrade(prev.Spec.Version, curr.Spec.Version)
}

func checkSingleConfigSource(l *Logstash) field.ErrorList {
	if l.Spec.Config != nil && l.Spec.ConfigRef != nil {
		msg := "Specify at most one of [`config`, `configRef`], not both"
		return field.ErrorList{
			field.Forbidden(field.NewPath("spec").Child("config"), msg),
			field.Forbidden(field.NewPath("spec").Child("configRef"), msg),
		}
	}

	return nil
}

func checkAssociations(l *Logstash) field.ErrorList {
	monitoringPath := field.NewPath("spec").Child("monitoring")
	err1 := commonv1.CheckAssociationRefs(monitoringPath.Child("metrics"), l.GetMonitoringMetricsRefs()...)
	err2 := commonv1.CheckAssociationRefs(monitoringPath.Child("logs"), l.GetMonitoringLogsRefs()...)
	err3 := commonv1.CheckAssociationRefs(field.NewPath("spec").Child("elasticsearchRefs"), l.ElasticsearchRefs()...)
	return append(append(err1, err2...), err3...)
}

func checkSinglePipelineSource(a *Logstash) field.ErrorList {
	if a.Spec.Pipelines != nil && a.Spec.PipelinesRef != nil {
		msg := "Specify at most one of [`pipelines`, `pipelinesRef`], not both"
		return field.ErrorList{
			field.Forbidden(field.NewPath("spec").Child("pipelines"), msg),
			field.Forbidden(field.NewPath("spec").Child("pipelinesRef"), msg),
		}
	}

	return nil
}

func checkESRefsNamed(l *Logstash) field.ErrorList {
	var errorList field.ErrorList
	for i, esRef := range l.Spec.ElasticsearchRefs {
		if esRef.ClusterName == "" {
			errorList = append(
				errorList,
				field.Required(
					field.NewPath("spec").Child("elasticsearchRefs").Index(i).Child("clusterName"),
					fmt.Sprintf("clusterName is a mandatory field - missing on %v", esRef.NamespacedName())),
			)
		}
	}
	return errorList
}

// checkPVCchanges ensures no PVCs are changed, as volume claim templates are immutable in StatefulSets.
func checkPVCchanges(current, proposed *Logstash) field.ErrorList {
	var errs field.ErrorList
	if current == nil || proposed == nil {
		return errs
	}

	// checking semantic equality here allows providing PVC storage size with different units (eg. 1Ti vs. 1024Gi).
	if !apiequality.Semantic.DeepEqual(current.Spec.VolumeClaimTemplates, proposed.Spec.VolumeClaimTemplates) {
		errs = append(errs, field.Invalid(field.NewPath("spec").Child("volumeClaimTemplates"), proposed.Spec.VolumeClaimTemplates, pvcImmutableMsg))
	}

	return errs
}
