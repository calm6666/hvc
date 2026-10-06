import { ref, ComputedRef, unref, computed, onMounted, watchEffect, watch } from 'vue';
import type { BasicTableProps } from '../types/table';
import type { PaginationProps } from '../types/pagination';
import { isBoolean, isFunction } from '@/utils/is';
import { APISETTING } from '../const';

/**
 * 表格数据源管理 Hook。
 *
 * 负责通过传入的 request 函数加载分页数据，并管理数据状态。
 *
 * 与后端分页响应对齐：
 *   - 后端返回 { page, pageSize, total, items }
 *   - pageCount（总页数）由前端根据 total / pageSize 向上取整计算
 */
export function useDataSource(
  propsRef: ComputedRef<BasicTableProps>,
  { getPaginationInfo, setPagination, setLoading, tableData },
  emit
) {
  const dataSourceRef = ref<Recordable[]>([]);

  watchEffect(() => {
    tableData.value = unref(dataSourceRef);
  });

  watch(
    () => unref(propsRef).dataSource,
    () => {
      const dataSource = unref(propsRef).dataSource;
      if (dataSource) {
        dataSourceRef.value = dataSource as Record<string, unknown>[];
      }
    },
    {
      immediate: true,
    }
  );

  const getRowKey = computed(() => {
    const rowKeyConfig = unref(propsRef).rowKey;
    return rowKeyConfig
      ? rowKeyConfig
      : () => 'key';
  });

  const getDataSourceRef = computed(() => {
    const dataSource = unref(dataSourceRef);
    if (!dataSource || dataSource.length === 0) {
      return unref(dataSourceRef);
    }
    return unref(dataSourceRef);
  });

  async function fetch(opt?) {
    try {
      setLoading(true);
      const { request, pagination, beforeRequest, afterRequest } = unref(propsRef);
      if (!request) return;

      // 组装分页信息
      const pageField = APISETTING.pageField;
      const sizeField = APISETTING.sizeField;
      const listField = APISETTING.listField;
      const countField = APISETTING.countField;

      let pageParams = {};
      const { page = 1, pageSize = 10 } = unref(getPaginationInfo) as PaginationProps;

      if ((isBoolean(pagination) && !pagination) || isBoolean(getPaginationInfo)) {
        // 分页关闭时不发送分页参数
        pageParams = {};
      } else {
        pageParams[pageField] = (opt && opt[pageField]) || page;
        pageParams[sizeField] = pageSize;
      }

      let params = {
        ...pageParams,
        ...opt,
      };

      // 请求前钩子：允许外部修改请求参数
      if (beforeRequest && isFunction(beforeRequest)) {
        params = (await beforeRequest(params)) || params;
      }

      const res = await request(params);

      // 从响应中提取分页字段（已由拦截器 toCamelCase 转换）
      const currentPage = res[pageField];        // "page" → 当前页码
      const total = res[countField];              // "total" → 总条数
      const results = res[listField] || [];       // "items" → 数据列表

      // 根据总条数和每页条数计算总页数（后端不返回 pageCount，前端自行计算）
      const pageCount = total > 0 ? Math.ceil(total / pageSize) : 0;

      // 页码越界修正：如果当前页大于总页数，重置为最后一页并重新请求
      if (pageCount > 0 && page > pageCount) {
        setPagination({
          page: pageCount,
          itemCount: total,
        });
        return await fetch(opt);
      }

      let resultInfo = results;
      // 请求后钩子：允许外部修改返回的数据
      if (afterRequest && isFunction(afterRequest)) {
        resultInfo = (await afterRequest(resultInfo)) || resultInfo;
      }

      dataSourceRef.value = resultInfo;
      setPagination({
        page: currentPage ?? page,
        pageCount,                                // 前端计算的总页数
        itemCount: total,                         // 总条数
      });

      // 如果外部显式传入了页码参数，覆盖分页状态
      if (opt && opt[pageField]) {
        setPagination({
          page: opt[pageField] || 1,
        });
      }
      emit('fetch-success', {
        items: unref(resultInfo),
        resultTotal: pageCount,
      });
    } catch (error) {
      console.error(error);
      emit('fetch-error', error);
      dataSourceRef.value = [];
      setPagination({
        pageCount: 0,
      });
    } finally {
      setLoading(false);
    }
  }

  onMounted(() => {
    setTimeout(() => {
      fetch();
    }, 16);
  });

  function setTableData(values) {
    dataSourceRef.value = values;
  }

  function getDataSource(): Record<string, unknown>[] {
    return getDataSourceRef.value;
  }

  async function reload(opt?) {
    await fetch(opt);
  }

  return {
    fetch,
    getRowKey,
    getDataSourceRef,
    getDataSource,
    setTableData,
    reload,
  };
}
