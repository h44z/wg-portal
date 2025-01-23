{{/*
Define the service template
{{- include "wg-portal.service" (dict "context" $ "scope" .Values.service.<name> "ports" list "name" "<name>") -}}
*/}}
{{- define "wg-portal.service.tpl" -}}
apiVersion: v1
kind: Service
metadata:
  {{- with .scope.annotations }}
  annotations: {{- toYaml . | nindent 4 }}
  {{- end }}
  labels: {{- include "wg-portal.labels" .context | nindent 4 }}
  name: {{ include "wg-portal.fullname" .context }}{{ ternary "" (printf "-%s" .name) (empty .name) }}
spec:
  {{- with .scope.clusterIP }}
  clusterIP: {{ . }}
  {{- end }}
  {{- with .scope.externalIPs }}
  externalIPs: {{ toYaml . | nindent 4 }}
  {{- end }}
  {{- with .scope.externalName }}
  externalName: {{ . }}
  {{- end }}
  {{- with .scope.externalTrafficPolicy }}
  externalTrafficPolicy: {{ . }}
  {{- end }}
  {{- with .scope.healthCheckNodePort }}
  healthCheckNodePort: {{ . }}
  {{- end }}
  {{- with .scope.loadBalancerIP }}
  loadBalancerIP: {{ . }}
  {{- end }}
  {{- with .scope.loadBalancerSourceRanges }}
  loadBalancerSourceRanges: {{ toYaml . | nindent 4 }}
  {{- end }}
  ports: {{- toYaml .ports | nindent 4 }}
  {{- with .scope.publishNotReadyAddresses }}
  publishNotReadyAddresses: {{ . }}
  {{- end }}
  {{- with .scope.sessionAffinity }}
  sessionAffinity: {{ . }}
  {{- end }}
  {{- with .scope.sessionAffinityConfig }}
  sessionAffinityConfig: {{ toYaml . | nindent 4 }}
  {{- end }}
  {{- with .scope.topologyKeys }}
  topologyKeys: {{ toYaml . | nindent 4 }}
  {{- end }}
  {{- with .scope.type }}
  type: {{ . }}
  {{- end }}
  selector: {{- include "wg-portal.selectorLabels" .context | nindent 4 }}
{{- end -}}

{{/*
Define the service port template for the web port
*/}}
{{- define "wg-portal.service.webPort" -}}
name: web
port: {{ .Values.service.web.port }}
protocol: TCP
targetPort: web
{{- if semverCompare ">=1.20-0" .Capabilities.KubeVersion.Version }}
appProtocol: {{ ternary "https" .Values.service.web.appProtocol .Values.certificate.enabled }}
{{- end -}}
{{- end -}}
