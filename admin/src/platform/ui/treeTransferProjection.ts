export type TreeTransferProjectionNode = {
  key: string;
  parentKey?: string;
};

export function treeTransferRootKeys(nodes: readonly TreeTransferProjectionNode[]) {
  const keys = new Set(nodes.map((node) => node.key));
  return nodes.filter((node) => !node.parentKey || !keys.has(node.parentKey)).map((node) => node.key).sort();
}
