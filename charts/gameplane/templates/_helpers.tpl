{{- define "gameplane.operatorImage" -}}
{{- printf "%s/operator:%s" .Values.image.registry .Values.image.tag -}}
{{- end -}}

{{- define "gameplane.apiImage" -}}
{{- printf "%s/api:%s" .Values.image.registry .Values.image.tag -}}
{{- end -}}

{{- define "gameplane.agentImage" -}}
{{- if .Values.operator.agentImage -}}
{{- .Values.operator.agentImage -}}
{{- else -}}
{{- printf "%s/agent:%s" .Values.image.registry .Values.image.tag -}}
{{- end -}}
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
