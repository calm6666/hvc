import { ComponentType } from './index';
import type { CSSProperties } from 'vue';
import type { GridProps, GridItemProps } from 'naive-ui/lib/grid';
import type { ButtonProps } from 'naive-ui/lib/button';

/** 表单字段描述 */
export interface FormSchema {
  /** 字段名（对应 formModel 的 key） */
  field: string;
  /** 标签文字 */
  label: string;
  /** 标签旁的提示信息 */
  labelMessage?: string;
  /** 提示信息样式 */
  labelMessageStyle?: Record<string, string>;
  /** 默认值 */
  defaultValue?: unknown;
  /** 渲染的组件类型 */
  component?: ComponentType;
  /** 组件属性（透传给渲染组件） */
  componentProps?: Record<string, unknown>;
  /** 自定义插槽名（slot 与 component 互斥） */
  slot?: string;
  /** 校验规则 */
  rules?: Record<string, unknown> | Record<string, unknown>[];
  /** 栅格项属性 */
  giProps?: GridItemProps;
  /** 是否占满宽度 */
  isFull?: boolean;
  /** 组件后置插槽名 */
  suffix?: string;
}

/** 表单组件 Props */
export interface FormProps {
  model?: Record<string, unknown>;
  labelWidth?: number | string;
  schemas?: FormSchema[];
  inline: boolean;
  layout?: string;
  size: string;
  labelPlacement: string;
  isFull: boolean;
  showActionButtonGroup?: boolean;
  showResetButton?: boolean;
  resetButtonOptions?: Partial<ButtonProps>;
  showSubmitButton?: boolean;
  showAdvancedButton?: boolean;
  submitButtonOptions?: Partial<ButtonProps>;
  submitButtonText?: string;
  resetButtonText?: string;
  gridProps?: GridProps;
  giProps?: GridItemProps;
  resetFunc?: () => Promise<void>;
  submitFunc?: () => Promise<void>;
  submitOnReset?: boolean;
  baseGridStyle?: CSSProperties;
  collapsedRows?: number;
}

/** 表单暴露给外部的操作方法 */
export interface FormActionType {
  /** 提交表单（触发校验 + 提交） */
  submit: () => Promise<Record<string, unknown> | boolean>;
  /** 动态设置表单 Props */
  setProps: (formProps: Partial<FormProps>) => Promise<void>;
  /** 动态设置 Schema */
  setSchema: (schemaProps: Partial<FormSchema[]>) => Promise<void>;
  /** 设置表单字段值 */
  setFieldsValue: (values: Record<string, unknown>) => void;
  /** 清除指定字段的校验状态 */
  clearValidate: (name?: string | string[]) => Promise<void>;
  /** 获取当前表单值 */
  getFieldsValue: () => Record<string, unknown>;
  /** 重置表单 */
  resetFields: () => Promise<void>;
  /** 校验表单 */
  validate: (nameList?: string[]) => Promise<Record<string, unknown>>;
  /** 设置加载状态 */
  setLoading: (status: boolean) => void;
}

/** useForm 注册函数类型 */
export type RegisterFn = (formInstance: FormActionType) => void;

/** useForm Hook 返回值 */
export type UseFormReturnType = [RegisterFn, FormActionType];
