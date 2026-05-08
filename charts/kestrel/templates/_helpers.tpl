{{- define "kestrel.operatorImage" -}}
{{- printf "%s/operator:%s" .Values.image.registry .Values.image.tag -}}
{{- end -}}

{{- define "kestrel.apiImage" -}}
{{- printf "%s/api:%s" .Values.image.registry .Values.image.tag -}}
{{- end -}}

{{- define "kestrel.agentImage" -}}
{{- if .Values.operator.agentImage -}}
{{- .Values.operator.agentImage -}}
{{- else -}}
{{- printf "%s/agent:%s" .Values.image.registry .Values.image.tag -}}
{{- end -}}
{{- end -}}

{{- /*
kestrel.labels intentionally omits app.kubernetes.io/name so each
resource can set its own (e.g. "kestrel-api", "kestrel-operator")
without colliding with Deployment selectors.
*/}}
{{- define "kestrel.labels" -}}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}
