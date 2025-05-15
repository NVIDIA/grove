{{- define "common.chart.labels" -}}
chart: "{{ .Chart.Name }}-{{ .Chart.Version }}"
release: "{{ .Release.Name }}"
{{- end -}}

{{- define "kube-scheduler.serviceaccount.labels" -}}
{{- include "common.chart.labels" . }}
{{- range $key, $val := .Values.serviceAccount.labels }}
{{ $key }}: {{ $val }}
{{- end }}
{{- end -}}

{{- define "kube-scheduler.rolebinding.labels" -}}
{{- include "common.chart.labels" . }}
{{- range $key, $val := .Values.roleBinding.labels }}
{{ $key }}: {{ $val }}
{{- end }}
{{- end -}}

{{- define "kube-scheduler.config.schedulerConfiguration" -}}
{{- range $key, $val := .Values.config.schedulerConfiguration }}
{{ $key }}: {{ $val }}
{{- end }}
{{- end -}}

{{- define "kube-scheduler.config.labels" -}}
{{- include "common.chart.labels" . }}
{{- range $key, $val := .Values.configMap.labels }}
{{ $key }}: {{ $val }}
{{- end }}
{{- end -}}

{{- define "kube-scheduler.clusterrolebinding.labels" -}}
{{- include "common.chart.labels" . }}
{{- range $key, $val := .Values.clusterRoleBinding.labels }}
{{ $key }}: {{ $val }}
{{- end }}
{{- end -}}

{{- define "kube-scheduler.deployment.labels" -}}
{{- include "common.chart.labels" . }}
{{- range $key, $val := .Values.deployment.labels }}
{{ $key }}: {{ $val }}
{{- end }}
{{- end -}}

{{- define "kube-scheduler.deployment.matchLabels" -}}
{{- range $key, $val := .Values.deployment.labels }}
{{ $key }}: {{ $val }}
{{- end }}
{{- end -}}

{{- define "image" -}}
{{- if hasPrefix "sha256:" (required "$.tag is required" $.tag) -}}
{{ required "$.repository is required" $.repository }}@{{ required "$.tag" $.tag }}
{{- else -}}
{{ required "$.repository is required" $.repository }}:{{ required "$.tag" $.tag }}
{{- end -}}
{{- end -}}
