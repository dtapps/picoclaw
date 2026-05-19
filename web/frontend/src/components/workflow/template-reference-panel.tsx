import { useState } from "react"
import { useTranslation } from "react-i18next"
import { IconChevronDown, IconChevronUp, IconFunction, IconAlertCircle, IconVariable, IconHash } from "@tabler/icons-react"

import { Card, CardHeader } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"

// 内置模板函数列表
const BUILTIN_TEMPLATE_FUNCTIONS = [
  { name: "now", example: '{{.fn.now}}', descKey: "func_current_utc_time" },
  { name: "now_tz", example: '{{.fn.now_tz "Asia/Shanghai"}}', descKey: "func_current_time_in_timezone" },
  { name: "date", example: '{{.fn.date}}', descKey: "func_current_utc_date" },
  { name: "date_tz", example: '{{.fn.date_tz "Asia/Shanghai"}}', descKey: "func_current_date_in_timezone" },
  { name: "unix", example: '{{.fn.unix}}', descKey: "func_unix_timestamp" },
  { name: "days_ago", example: '{{.fn.days_ago 7}}', descKey: "func_days_ago" },
  { name: "days_from_now", example: '{{.fn.days_from_now 3}}', descKey: "func_days_from_now" },
  { name: "hours_ago", example: '{{.fn.hours_ago 24}}', descKey: "func_hours_ago" },
  { name: "hours_from_now", example: '{{.fn.hours_from_now 2}}', descKey: "func_hours_from_now" },
  { name: "minutes_ago", example: '{{.fn.minutes_ago 30}}', descKey: "func_minutes_ago" },
  { name: "minutes_from_now", example: '{{.fn.minutes_from_now 15}}', descKey: "func_minutes_from_now" },
  { name: "weeks_ago", example: '{{.fn.weeks_ago 2}}', descKey: "func_weeks_ago" },
  { name: "day_of_week", example: '{{.fn.day_of_week}}', descKey: "func_day_of_week" },
  { name: "format_time", example: '{{.fn.format_time "2006-01-02"}}', descKey: "func_format_time" },
  { name: "env", example: '{{.fn.env "HOME"}}', descKey: "func_environment_variable" },
]

// 步骤状态字段
const STEP_STATUS_FIELDS = [
  { name: "_status", example: "{{.step_id._status}}", descKey: "step_status", values: "completed | failed" },
  { name: "_error", example: "{{.step_id._error}}", descKey: "step_error_message", values: "error text | empty" },
]

// 变量引用
const VAR_REFERENCE = { name: "vars", example: "{{.vars.variable_name}}", descKey: "workflow_variables_desc" }

// self 引用
const SELF_REFERENCES = [
  { name: "self.id", example: "{{.self.id}}", descKey: "current_step_id" },
  { name: "self.name", example: "{{.self.name}}", descKey: "current_step_name" },
]

interface TemplateReferencePanelProps {
  className?: string
}

export function TemplateReferencePanel({ className }: TemplateReferencePanelProps) {
  const { t } = useTranslation()
  const [isOpen, setIsOpen] = useState(true)
  const [funcOpen, setFuncOpen] = useState(false)
  const [statusOpen, setStatusOpen] = useState(true)
  const [varOpen, setVarOpen] = useState(false)

  return (
    <Card className={className}>
      <CardHeader className="pb-3">
        {/* 主标题 */}
        <button
          onClick={() => setIsOpen(!isOpen)}
          className="flex w-full items-center justify-between text-sm font-medium"
        >
          <span className="flex items-center gap-2">
            <IconFunction className="h-4 w-4" />
            {t("pages.workflows.template_references")}
          </span>
          {isOpen ? <IconChevronUp className="h-4 w-4" /> : <IconChevronDown className="h-4 w-4" />}
        </button>

        {/* 内容区域 */}
        {isOpen && (
          <div className="mt-3 space-y-3">
            {/* 模板函数 */}
            <div>
              <button
                onClick={() => setFuncOpen(!funcOpen)}
                className="flex w-full items-center justify-between text-xs font-medium text-muted-foreground hover:text-foreground"
              >
                <span className="flex items-center gap-1.5">
                  <IconHash className="h-3 w-3" />
                  {t("pages.workflows.template_functions")}
                </span>
                {funcOpen ? <IconChevronUp className="h-3 w-3" /> : <IconChevronDown className="h-3 w-3" />}
              </button>
              {funcOpen && (
                <div className="mt-2 space-y-1.5">
                  {BUILTIN_TEMPLATE_FUNCTIONS.map((fn) => (
                    <div
                      key={fn.name}
                      className="rounded-md border px-2 py-1.5 text-xs hover:bg-muted/50"
                    >
                      <code className="font-mono text-primary">{fn.example}</code>
                      <span className="ml-2 text-[10px] text-muted-foreground">
                        {t(`pages.workflows.${fn.descKey}`)}
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </div>

            {/* 步骤状态 */}
            <div>
              <button
                onClick={() => setStatusOpen(!statusOpen)}
                className="flex w-full items-center justify-between text-xs font-medium text-muted-foreground hover:text-foreground"
              >
                <span className="flex items-center gap-1.5">
                  <IconAlertCircle className="h-3 w-3" />
                  {t("pages.workflows.step_status_fields")}
                </span>
                {statusOpen ? <IconChevronUp className="h-3 w-3" /> : <IconChevronDown className="h-3 w-3" />}
              </button>
              {statusOpen && (
                <div className="mt-2 space-y-1.5">
                  {STEP_STATUS_FIELDS.map((field) => (
                    <div
                      key={field.name}
                      className="rounded-md border px-2 py-1.5 text-xs hover:bg-muted/50"
                    >
                      <code className="font-mono text-primary">{field.example}</code>
                      <Badge variant="secondary" className="ml-2 h-4 text-[9px]">
                        {field.values}
                      </Badge>
                      <span className="ml-2 text-[10px] text-muted-foreground">
                        {t(`pages.workflows.${field.descKey}`)}
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </div>

            {/* 变量和 self */}
            <div>
              <button
                onClick={() => setVarOpen(!varOpen)}
                className="flex w-full items-center justify-between text-xs font-medium text-muted-foreground hover:text-foreground"
              >
                <span className="flex items-center gap-1.5">
                  <IconVariable className="h-3 w-3" />
                  {t("pages.workflows.variables_and_self")}
                </span>
                {varOpen ? <IconChevronUp className="h-3 w-3" /> : <IconChevronDown className="h-3 w-3" />}
              </button>
              {varOpen && (
                <div className="mt-2 space-y-1.5">
                  {/* vars */}
                  <div className="rounded-md border px-2 py-1.5 text-xs hover:bg-muted/50">
                    <code className="font-mono text-primary">{VAR_REFERENCE.example}</code>
                    <span className="ml-2 text-[10px] text-muted-foreground">
                      {t(`pages.workflows.${VAR_REFERENCE.descKey}`)}
                    </span>
                  </div>
                  {/* self */}
                  {SELF_REFERENCES.map((ref) => (
                    <div
                      key={ref.name}
                      className="rounded-md border px-2 py-1.5 text-xs hover:bg-muted/50"
                    >
                      <code className="font-mono text-primary">{ref.example}</code>
                      <span className="ml-2 text-[10px] text-muted-foreground">
                        {t(`pages.workflows.${ref.descKey}`)}
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        )}
      </CardHeader>
    </Card>
  )
}
