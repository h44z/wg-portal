{{- define "wg-portal.podTemplate" -}}
metadata:
  annotations:
    checksum/config: {{ include (print $.Template.BasePath "/secret.yaml") . | sha256sum }}
    kubectl.kubernetes.io/default-container: {{ .Chart.Name }}
    {{- with .Values.podAnnotations }}
    {{- tpl (toYaml .) $ | nindent 4 }}
    {{- end }}
  labels: {{- include "wg-portal.util.merge" (list $ .Values.podLabels "wg-portal.selectorLabels") | nindent 4 }}
spec:
  {{- with .Values.affinity }}
  affinity: {{- toYaml . | nindent 4 }}
  {{- end }}
  automountServiceAccountToken: {{ .Values.serviceAccount.automount }}
  containers:
    {{- with .Values.sidecarContainers }}
    {{- tpl (toYaml .) $ | nindent 4 }}
    {{- end }}
    - name: {{ .Chart.Name }}
      image: "{{ .Values.image.repository }}:{{ default .Chart.AppVersion .Values.image.tag}}"
      imagePullPolicy: {{ .Values.image.pullPolicy }}
      {{- with .Values.command }}
      command: {{ . }}
      {{- end }}
      {{- with .Values.args }}
      args: {{ . }}
      {{- end }}
      {{- with .Values.env }}
      env: {{- tpl (toYaml .) $ | nindent 8 }}
      {{- end }}
      {{- with .Values.envFrom }}
      envFrom: {{- tpl (toYaml .) $ | nindent 8 }}
      {{- end }}
      ports:
        - name: metrics
          containerPort: {{ .Values.service.metrics.port}}
          protocol: TCP
        - name: web
          containerPort: {{ .Values.service.web.port }}
          protocol: TCP
        {{- range $index, $port := .Values.service.wireguard.ports }}
        - name: wg{{ $index }}
          containerPort: {{ $port }}
          protocol: UDP
        {{- end }}
      {{- with .Values.livenessProbe }}
      livenessProbe: {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.readinessProbe }}
      readinessProbe: {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.startupProbe }}
      startupProbe: {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.securityContext }}
      securityContext: {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.resources}}
      resources: {{- toYaml . | nindent 8 }}
      {{- end }}
      volumeMounts:
        - name: config
          mountPath: /app/config
          readOnly: true
        - name: data
          mountPath: /app/data
        {{- if and .Values.certificate.enabled (include "wg-portal.hostname" .) }}
        - name: certs
          mountPath: /app/certs
        {{- end }}
        {{- with .Values.volumeMounts }}
        {{- tpl (toYaml .) $ | nindent 8 }}
        {{- end }}
  {{- with .Values.dnsPolicy }}
  dnsPolicy: {{ . }}
  {{- end }}
  {{- with .Values.hostNetwork }}
  hostNetwork: {{ . }}
  {{- end }}
  {{- with .Values.imagePullSecrets }}
  imagePullSecrets: {{- toYaml . | nindent 4 }}
  {{- end }}
  {{- with .Values.initContainers }}
  initContainers: {{- tpl (toYaml .) $ | nindent 4 }}
  {{- end }}
  {{- with .Values.nodeSelector }}
  nodeSelector: {{- toYaml . | nindent 4 }}
  {{- end }}
  {{- with .Values.restartPolicy }}
  restartPolicy: {{ . }}
  {{- end }}
  serviceAccountName: {{ include "wg-portal.serviceAccountName" . }}
  {{- with .Values.podSecurityContext }}
  securityContext: {{- toYaml . | nindent 4 }}
  {{- end }}
  {{- with .Values.tolerations }}
  tolerations: {{- toYaml . | nindent 4 }}
  {{- end }}
  volumes:
    - name: config
      secret:
        secretName: {{ include "wg-portal.fullname" . }}
    {{- if and .Values.certificate.enabled (include "wg-portal.hostname" .) }}
    - name: certs
      secret:
        secretName: {{ include "wg-portal.fullname" . }}-tls
    {{- end }}
    {{- if not .Values.persistence.enabled }}
    - name: data
      emptyDir: {}
    {{- else if eq .Values.workloadType "Deployment" }}
    - name: data
      persistentVolumeClaim:
        claimName: {{ include "wg-portal.fullname" . }}
    {{- end }}
    {{- with .Values.volumes }}
    {{- tpl (toYaml .) $ | nindent 4 }}
    {{- end }}
{{- end -}}
