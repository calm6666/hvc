<template>
  <div>
    <div class="n-layout-page-header"><n-card :bordered="false" title="运行配置" /></div>
    <n-card :bordered="false" class="mt-4 proCard" size="small">
      <n-tabs type="line" animated @update:value="onTabChange">
        <n-tab-pane v-for="tab in tabs" :key="tab.key" :name="tab.key" :tab="tab.label" />
        <!-- 表单区域 -->
        <div class="pt-4">
          <n-form :model="formModel" label-placement="left" :label-width="180" size="small">
            <n-form-item v-for="field in currentFields" :key="field.key" :label="field.label">
              <n-switch v-if="field.type==='switch'" v-model:value="formModel[field.key]" />
              <n-input-number v-else-if="field.type==='number'" v-model:value="formModel[field.key]" style="width:200px" />
              <n-input v-else v-model:value="formModel[field.key]" style="width:320px" />
            </n-form-item>
          </n-form>
          <n-button type="primary" :loading="submitting" @click="handleSubmit" class="mt-3">提交变更</n-button>
          <span class="text-gray-400 ml-3 text-sm">修改后需通过「版本管理」页发布才能生效</span>
        </div>
      </n-tabs>
    </n-card>
  </div>
</template>

<script lang="ts" setup>
  import { ref, computed, reactive } from 'vue';
  import { useMessage } from 'naive-ui';
  import { updateRuntimeServer, updateRuntimeScheduler, updateRuntimeWorker, updateRuntimeStorage, updateRuntimeCallback, updateRuntimeMq, updateRuntimeGrpc } from '@/api/config';

  const message = useMessage();
  const submitting = ref(false);
  const activeTab = ref('server');

  /** 表单字段配置 */
  interface FieldConfig { key: string; label: string; type: 'text' | 'number' | 'switch'; defaultValue: unknown; }

  /** 各 Tab 的表单字段定义 */
  const tabFields: Record<string, FieldConfig[]> = {
    server: [
      { key: 'enableHttpServer', label: '启用 HTTP Server', type: 'switch', defaultValue: true },
      { key: 'enableCallback', label: '启用回调', type: 'switch', defaultValue: false },
    ],
    scheduler: [
      { key: 'schedulerLoopIntervalMs', label: '调度循环间隔(ms)', type: 'number', defaultValue: 2000 },
      { key: 'maxGlobalTranscodeSessions', label: '全局最大转码会话数', type: 'number', defaultValue: 40 },
      { key: 'jobLeaseTtlSec', label: '任务租约 TTL(秒)', type: 'number', defaultValue: 60 },
      { key: 'requireHardwareEncode', label: '强制硬件编码', type: 'switch', defaultValue: false },
      { key: 'allowSoftwareDecodeFallback', label: '允许软解码回退', type: 'switch', defaultValue: true },
      { key: 'requireHardwareWatermark', label: '强制硬件水印', type: 'switch', defaultValue: false },
      { key: 'nodeCpuSafetyLimitPercent', label: '节点 CPU 安全上限(%)', type: 'number', defaultValue: 80 },
      { key: 'nodeMemorySafetyLimitPercent', label: '节点内存安全上限(%)', type: 'number', defaultValue: 80 },
      { key: 'nodeGpuSafetyLimitPercent', label: '节点 GPU 安全上限(%)', type: 'number', defaultValue: 80 },
    ],
    worker: [
      { key: 'workerHeartbeatTimeoutSec', label: 'Worker 心跳超时(秒)', type: 'number', defaultValue: 30 },
      { key: 'workerLoopIntervalMs', label: 'Worker 循环间隔(ms)', type: 'number', defaultValue: 1000 },
      { key: 'singleJobUploadConcurrency', label: '单任务上传并发数', type: 'number', defaultValue: 4 },
    ],
    storage: [
      { key: 'storageType', label: '存储类型', type: 'text', defaultValue: 's3' },
      { key: 'storageEndpoint', label: '存储端点', type: 'text', defaultValue: '' },
      { key: 'storageBucket', label: 'Bucket', type: 'text', defaultValue: '' },
      { key: 'storageAccessKeyId', label: 'AccessKey ID', type: 'text', defaultValue: '' },
      { key: 'storageSecretAccessKey', label: 'SecretAccessKey', type: 'text', defaultValue: '' },
      { key: 'storageUseSsl', label: '使用 SSL', type: 'switch', defaultValue: true },
      { key: 'storagePlayDomain', label: '播放域名', type: 'text', defaultValue: '' },
      { key: 'storageFlvDomain', label: 'FLV 播放域名', type: 'text', defaultValue: '' },
      { key: 'storageLocalBasePath', label: '本地存储路径', type: 'text', defaultValue: '/data/hvc' },
      { key: 'defaultStorageId', label: '默认存储配置 ID', type: 'number', defaultValue: 0 },
    ],
    callback: [
      { key: 'callbackHttpUrl', label: '回调 HTTP URL', type: 'text', defaultValue: '' },
      { key: 'callbackRpcEndpoint', label: '回调 RPC Endpoint', type: 'text', defaultValue: '' },
      { key: 'callbackMqTopic', label: '回调 MQ Topic', type: 'text', defaultValue: '' },
    ],
    mq: [
      { key: 'enableMqConsumer', label: '启用 MQ 消费者', type: 'switch', defaultValue: false },
      { key: 'mqHost', label: 'MQ Host', type: 'text', defaultValue: 'localhost' },
      { key: 'mqPort', label: 'MQ 端口', type: 'number', defaultValue: 5672 },
      { key: 'mqUsername', label: 'MQ 用户名', type: 'text', defaultValue: 'guest' },
      { key: 'mqPassword', label: 'MQ 密码', type: 'text', defaultValue: '' },
      { key: 'mqVhost', label: 'MQ VHost', type: 'text', defaultValue: '/' },
      { key: 'mqQueueName', label: '队列名称', type: 'text', defaultValue: 'hvc_jobs' },
      { key: 'mqPrefetchCount', label: '预取数量', type: 'number', defaultValue: 10 },
    ],
    grpc: [
      { key: 'enableGrpcServer', label: '启用 gRPC Server', type: 'switch', defaultValue: true },
      { key: 'grpcListenAddress', label: '监听地址', type: 'text', defaultValue: ':19090' },
      { key: 'publicGrpcRegistryId', label: '公共 gRPC 注册中心 ID', type: 'number', defaultValue: 0 },
    ],
  };

  const tabs = [
    { key: 'server', label: 'Server' }, { key: 'scheduler', label: 'Scheduler' },
    { key: 'worker', label: 'Worker' }, { key: 'storage', label: 'Storage' },
    { key: 'callback', label: 'Callback' }, { key: 'mq', label: 'MQ' }, { key: 'grpc', label: 'gRPC' },
  ];

  /** 当前 Tab 的表单字段列表 */
  const currentFields = computed<FieldConfig[]>(() => tabFields[activeTab.value] || []);

  /** 表单数据模型 */
  const formModel = reactive<Record<string, unknown>>({});

  function onTabChange(key: string): void {
    activeTab.value = key;
    // 重置表单为当前 Tab 的默认值
    for (const k of Object.keys(formModel)) delete formModel[k];
    for (const f of tabFields[key]) { formModel[f.key] = f.defaultValue; }
  }
  // 初始化默认值
  onTabChange('server');

  /** API 映射 */
  const apiMap: Record<string, (data: Record<string, unknown>) => Promise<unknown>> = {
    server: updateRuntimeServer, scheduler: updateRuntimeScheduler, worker: updateRuntimeWorker,
    storage: updateRuntimeStorage, callback: updateRuntimeCallback, mq: updateRuntimeMq, grpc: updateRuntimeGrpc,
  };

  async function handleSubmit(): Promise<void> {
    submitting.value = true;
    try {
      const data = { ...formModel, changeSummary: `更新 ${activeTab.value} 配置` };
      await apiMap[activeTab.value](data);
      message.success(`${activeTab.value} 配置已提交，发布后生效`);
    } catch (e: unknown) {
      if (e instanceof Error) message.error(e.message);
    } finally { submitting.value = false; }
  }
</script>
