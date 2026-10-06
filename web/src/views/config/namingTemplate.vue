<template>
  <div>
    <div class="n-layout-page-header"><n-card :bordered="false" title="命名模板" /></div>
    <n-card :bordered="false" class="mt-4 proCard">
      <BasicTable :columns="columns" :request="loadDataTable" :row-key="(row: NamingTemplateRow) => row.id" ref="actionRef"
        :scroll-x="tableScrollX" :actionColumn="actionColumn" />
    </n-card>
    <basicModal @register="modalRegister" @on-ok="handleSubmit"><BasicForm @register="registerForm" class="pt-5" /></basicModal>
  </div>
</template>

<script lang="ts" setup>
  import { computed, h, ref, reactive } from 'vue';
  import { NTag, useMessage } from 'naive-ui';
  import { BasicTable, TableAction } from '@/components/Table';
  import { basicModal, useModal } from '@/components/Modal';
  import { BasicForm, useForm } from '@/components/Form';
  import { listNamingTemplates, configureNamingTemplate, activateNamingTemplate } from '@/api/config';

  const message = useMessage();
  const actionRef = ref();

  interface NamingTemplateRow { id: number; name: string; template: string; exampleInitVideo: string; description: string; }

  const columns = [
    { title: 'ID', key: 'id', width: 60 },
    { title: '名称', key: 'name', width: 140 },
    { title: '模板', key: 'template', width: 300, ellipsis: { tooltip: true } },
    { title: '示例', key: 'exampleInitVideo', width: 200, ellipsis: { tooltip: true } },
    { title: '描述', key: 'description', width: 200 },
  ];

  const actionColumn = reactive({
    width: 180, title: '操作', key: 'action', fixed: 'right' as const,
    render(record: NamingTemplateRow) { return h(TableAction, { style: 'button', actions: [
      { label: '配置', auth: ['config.naming_template.update'], onClick: () => handleEdit(record) },
      { label: '激活', auth: ['config.naming_template.update'], onClick: () => doActivate(record) },
    ]});},
  });

  const loadDataTable = async (res: Record<string, unknown>) => await listNamingTemplates(res as Parameters<typeof listNamingTemplates>[0]);
  const tableScrollX = computed(() => columns.reduce((s, c) => s + ((c.width as number) || 150), 0));
  function reloadTable() { actionRef.value.reload(); }

  const schemas = [
    { field: 'templateId', label: '模板 ID', component: 'NInputNumber', defaultValue: 1 },
    { field: 'customTemplate', label: '自定义模板', component: 'NInput' },
  ];
  const [registerForm, { submit, setFieldsValue }] = useForm({ schemas, labelWidth: 100, layout: 'horizontal', showActionButtonGroup: false, gridProps: { cols: 1 } });
  const [modalRegister, { openModal, closeModal, setSubLoading }] = useModal({ title: '配置命名模板', subBtuText: '保存' });

  function handleEdit(row: NamingTemplateRow) { setFieldsValue({ templateId: row.id, customTemplate: row.template }); openModal(); }
  async function handleSubmit() {
    try { await submit(); await configureNamingTemplate(formMethods.getFieldsValue()); message.success('配置已提交，需发布后生效'); closeModal(); reloadTable(); }
    catch (e: unknown) { if (e instanceof Error) message.error(e.message); setSubLoading(false); }
  }
  async function doActivate(row: NamingTemplateRow) { await activateNamingTemplate({ template: row.template }); message.success('模板已激活'); reloadTable(); }
</script>
