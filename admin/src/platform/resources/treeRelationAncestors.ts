export async function hydrateTreeRelationAncestors<TRecord>(
  records: readonly TRecord[],
  valueOf: (record: TRecord) => string,
  parentValueOf: (record: TRecord) => string,
  loadByValue: (value: string) => Promise<TRecord | undefined>,
) {
  const result = [...records];
  const loadedValues = new Set(records.map((record) => valueOf(record).trim()).filter(Boolean));
  const attemptedValues = new Set<string>();
  const pendingValues = [...new Set(records.map((record) => parentValueOf(record).trim()).filter(Boolean))];

  while (pendingValues.length > 0) {
    const value = pendingValues.shift() as string;
    if (loadedValues.has(value) || attemptedValues.has(value)) continue;
    attemptedValues.add(value);

    const ancestor = await loadByValue(value);
    if (!ancestor) continue;
    const ancestorValue = valueOf(ancestor).trim();
    if (!ancestorValue || ancestorValue !== value) continue;

    result.push(ancestor);
    loadedValues.add(ancestorValue);
    const parentValue = parentValueOf(ancestor).trim();
    if (parentValue && !loadedValues.has(parentValue) && !attemptedValues.has(parentValue)) {
      pendingValues.push(parentValue);
    }
  }

  return result;
}
