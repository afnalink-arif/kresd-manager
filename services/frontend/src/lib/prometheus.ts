// Extract scalar value from Prometheus instant query response
export function extractValue(promResponse: any): number | null {
  if (!promResponse) return null;

  // Direct number
  if (typeof promResponse === "number") return promResponse;

  // Prometheus API response: { status, data: { resultType, result: [{ value: [ts, "val"] }] } }
  const result = promResponse?.data?.result;
  if (Array.isArray(result) && result.length > 0) {
    const val = result[0]?.value?.[1];
    if (val !== undefined) return parseFloat(val);
  }

  // Nested in data directly
  if (promResponse?.result) {
    const r = promResponse.result;
    if (Array.isArray(r) && r.length > 0) {
      const val = r[0]?.value?.[1];
      if (val !== undefined) return parseFloat(val);
    }
  }

  return null;
}

// Extract time series from Prometheus range query response
export function extractTimeSeries(promResponse: any): { timestamps: number[]; values: number[] } {
  const empty = { timestamps: [], values: [] };
  if (!promResponse) return empty;

  const result = promResponse?.data?.result;
  if (!Array.isArray(result) || result.length === 0) return empty;

  // range query: result[0].values = [[ts, "val"], ...]
  const values = result[0]?.values;
  if (!Array.isArray(values)) return empty;

  return {
    timestamps: values.map((v: any) => v[0]),
    values: values.map((v: any) => parseFloat(v[1]) || 0),
  };
}

// Format a number nicely
export function fmt(val: number | null, decimals = 1): string {
  if (val === null || isNaN(val)) return "--";
  return val.toFixed(decimals);
}
