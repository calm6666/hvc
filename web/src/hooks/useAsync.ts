import { isReactive, isRef } from 'vue';

function setLoading(loading: unknown, val: boolean): void {
  if (loading != undefined && isRef(loading)) {
    (loading as { value: boolean }).value = val;
  } else if (loading != undefined && isReactive(loading)) {
    (loading as { loading: boolean }).loading = val;
  }
}

export const useAsync = async <T>(func: Promise<T>, loading: unknown): Promise<T> => {
  setLoading(loading, true);
  return await func.finally(() => setLoading(loading, false));
};
