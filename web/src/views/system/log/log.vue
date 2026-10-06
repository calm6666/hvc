<template>
  <div>
    <div class="n-layout-page-header">
      <n-card :bordered="false" title="运行日志" />
    </div>
    <n-card :bordered="false" class="mt-4 proCard">
      <!--
        运行日志列表。
        支持按日志级别（info/warn/error）筛选，筛选条件通过 loadDataTable 拼入请求参数。
        字段与后端 runtimeLogView（经 toCamelCase 转换）对齐：
          logId, serviceName, logLevel, actionName, fields (map), loggedAt
      -->
      <BasicTable
        :columns="columns"
        :request="loadDataTable"
        :row-key="(row: RuntimeLogRow) => row.logId"
        ref="actionRef"
        :scroll-x="tableScrollX"
      >
        <template #toolbar>
          <n-space>
            <n-select
              v-model:value="logLevel"
              :options="levelOptions"
              placeholder="日志级别"
              clearable
              style="width: 120px"
              @update:value="reloadTable"
            />
          </n-space>
        </template>
      </BasicTable>
    </n-card>
  </div>
</template>

<script lang="ts" setup>
  import { computed, h, ref } from 'vue';
  import { NTag } from 'naive-ui';
  import { BasicTable } from '@/components/Table';
  import { listRuntimeLogs } from '@/api/system/log';

  const actionRef = ref();

  /** 当前筛选的日志级别：null 表示不过滤 */
  const logLevel = ref<string | null>(null);

  const levelOptions = [
    { label: 'INFO', value: 'info' },
    { label: 'WARN', value: 'warn' },
    { label: 'ERROR', value: 'error' },
  ];

  /**
   * 运行日志行数据类型。
   * 字段与后端 runtimeLogView（经拦截器 toCamelCase 转换后）对齐：
   *   LogID → logId
   *   Service → serviceName
   *   Level → logLevel
   *   ActionName → actionName
   *   Fields → fields        // map[string]any
   *   LoggedAt → loggedAt
   */
  interface RuntimeLogRow {
    logId: number;
    serviceName: string;
    logLevel: string;
    actionName: string;
    fields: Record<string, unknown> | string | null;
    loggedAt: string;
  }

  const columns = [
    { title: 'ID', key: 'logId', width: 80 },
    { title: '服务', key: 'serviceName', width: 120 },
    {
      title: '级别', key: 'logLevel', width: 80,
      /** 日志级别 → NTag 颜色映射：error=红色, warn=黄色, info/其他=蓝色 */
      render: (row: RuntimeLogRow) => {
        const type =
          row.logLevel === 'error' ? 'error' :
          row.logLevel === 'warn' ? 'warning' : 'info';
        return h(NTag, { type }, () => (row.logLevel || '').toUpperCase());
      },
    },
    { title: '操作', key: 'actionName', width: 160 },
    {
      title: '详情', key: 'fields', width: 300,
      /** fields 为 map 或 JSON 字符串，统一序列化为字符串展示 */
      render: (row: RuntimeLogRow) => {
        const f = row.fields;
        if (!f) return '';
        return typeof f === 'string' ? f : JSON.stringify(f);
      },
    },
    { title: '时间', key: 'loggedAt', width: 170 },
  ];

  /**
   * 表格数据加载函数，供 BasicTable 的 :request 属性调用。
   * 将当前筛选的日志级别（logLevel）拼入查询参数。
   */
  const loadDataTable = async (res: Record<string, unknown>) =>
    await listRuntimeLogs({
      ...res,
      level: logLevel.value || undefined,
    } as Parameters<typeof listRuntimeLogs>[0]);
  const tableScrollX = computed(() => columns.reduce((s, c) => s + ((c.width as number) || 150), 0));

  /** 刷新表格数据 */
  function reloadTable(): void {
    actionRef.value?.reload();
  }
</script>
