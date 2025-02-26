<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { useEventSource } from '@vueuse/core'

import Node from '@/components/Node.vue'

interface Node {
  name: string
  pods: Pod[]
}

interface Pod {
  name: string
  status: string
}

const nodes = ref<Node[]>([])

onMounted(() => {
  const { data } = useEventSource('http://localhost:8080/state', ['updated'] as const)
  watch(data, (data) => {
    if (!data) return
    try {
      const clusterData = JSON.parse(data) // Expecting { nodes: [{ name: 'node1', pods: [{ name: 'pod1' }] }] }
      console.log('Received SSE data:', clusterData)

      nodes.value = clusterData.nodes.map((node: any) => ({
        name: node.name,
        pods: node.pods || [],
      }))
    } catch (error) {
      console.error('Error parsing SSE data:', error)
    }
  })
})
</script>

<template>
  <div class="cluster-container">
    <Node v-for="node in nodes" :key="node.name" :nodeName="node.name" :pods="node.pods" />
  </div>
</template>

<style scoped>
.cluster-container {
  display: flex;
  flex-wrap: wrap;
  gap: 20px;
}
</style>
