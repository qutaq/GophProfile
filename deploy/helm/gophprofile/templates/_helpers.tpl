{{/*
Expand the name of the chart.
*/}}
{{- define "gophprofile.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "gophprofile.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{- define "gophprofile.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "gophprofile.labels" -}}
helm.sh/chart: {{ include "gophprofile.chart" . }}
{{ include "gophprofile.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
{{- end }}

{{- define "gophprofile.selectorLabels" -}}
app.kubernetes.io/name: {{ include "gophprofile.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "gophprofile.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "gophprofile.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "gophprofile.secretName" -}}
{{- if .Values.existingSecret }}
{{- .Values.existingSecret }}
{{- else }}
{{- include "gophprofile.fullname" . }}
{{- end }}
{{- end }}

{{- define "gophprofile.image" -}}
{{- $tag := .Values.image.tag | default .Chart.AppVersion }}
{{- printf "%s:%s" .Values.image.repository $tag }}
{{- end }}

{{- define "gophprofile.podSecurityContext" -}}
{{- toYaml .Values.podSecurityContext }}
{{- end }}

{{- define "gophprofile.containerSecurityContext" -}}
{{- toYaml .Values.securityContext }}
{{- end }}

{{- define "gophprofile.envFrom" -}}
envFrom:
  - configMapRef:
      name: {{ include "gophprofile.fullname" . }}
  - secretRef:
      name: {{ include "gophprofile.secretName" . }}
{{- end }}

{{- define "gophprofile.tmpVolume" -}}
volumes:
  - name: tmp
    emptyDir: {}
{{- end }}

{{- define "gophprofile.tmpVolumeMount" -}}
volumeMounts:
  - name: tmp
    mountPath: /tmp
{{- end }}

{{/*
Egress peer for NetworkPolicy. Pass a dict with port, podLabels, namespaceLabels.
*/}}
{{- define "gophprofile.netpolEgress" -}}
- to:
    {{- if and .namespaceLabels (gt (len .namespaceLabels) 0) }}
    - namespaceSelector:
        matchLabels:
          {{- toYaml .namespaceLabels | nindent 10 }}
      {{- if .podLabels }}
      podSelector:
        matchLabels:
          {{- toYaml .podLabels | nindent 10 }}
      {{- end }}
    {{- else }}
    - podSelector:
        matchLabels:
          {{- toYaml .podLabels | nindent 10 }}
    {{- end }}
  ports:
    - protocol: TCP
      port: {{ .port }}
{{- end }}
