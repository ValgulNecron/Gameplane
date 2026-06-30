{{- /*
gameplane.imageTag resolves the shared image tag: an explicit image.tag wins,
otherwise it falls back to the chart's appVersion so a released chart vX always
pulls images vX with no per-install override. Set image.tag (e.g. "edge") to
track a rolling channel.
*/}}
{{- define "gameplane.imageTag" -}}
{{- .Values.image.tag | default .Chart.AppVersion -}}
{{- end -}}

{{- define "gameplane.operatorImage" -}}
{{- printf "%s/operator:%s" .Values.image.registry (include "gameplane.imageTag" .) -}}
{{- end -}}

{{- define "gameplane.apiImage" -}}
{{- printf "%s/api:%s" .Values.image.registry (include "gameplane.imageTag" .) -}}
{{- end -}}

{{- define "gameplane.agentImage" -}}
{{- if .Values.operator.agentImage -}}
{{- .Values.operator.agentImage -}}
{{- else -}}
{{- printf "%s/agent:%s" .Values.image.registry (include "gameplane.imageTag" .) -}}
{{- end -}}
{{- end -}}

{{- define "gameplane.auditSyslogBridgeImage" -}}
{{- printf "%s/audit-syslog-bridge:%s" .Values.image.registry (include "gameplane.imageTag" .) -}}
{{- end -}}

{{- /*
gameplane.labels intentionally omits app.kubernetes.io/name so each
resource can set its own (e.g. "gameplane-api", "gameplane-operator")
without colliding with Deployment selectors.
*/}}
{{- define "gameplane.labels" -}}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}
