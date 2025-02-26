<template>
  <div class="node-box">
    <div class="node-header">
      {{ nodeName }}
    </div>
    <div class="node-body">
      <div v-if="pods.length === 0" class="no-pods">No pods running</div>
      <div v-for="pod in pods" :key="pod.name" class="pod-box" :class="getPodClass(pod.status)">
        {{ pod.name }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  nodeName: string
  pods: {
    name: string
    status: string
  }[]
}>()

// Function to determine pod color class
const getPodClass = (status: string) => {
  switch (status.toLowerCase()) {
    case 'running':
      return 'pod-running'
    case 'pending':
      return 'pod-pending'
    case 'failed':
      return 'pod-failed'
    case 'succeeded':
      return 'pod-succeeded'
    default:
      return 'pod-unknown'
  }
}
</script>

<style scoped>
.node-box {
  border: 2px solid #333;
  border-radius: 8px;
  width: 300px;
  margin: 10px;
  padding: 10px;
  box-shadow: 2px 2px 8px rgba(0, 0, 0, 0.2);
}

.node-header {
  font-weight: bold;
  padding: 10px;
  text-align: center;
  border-radius: 4px;
  font-size: 1.2em;
  background-color: var(--dark-blue);
  color: var(--off-white);
}

.node-body {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  padding: 10px;
  justify-content: center;
}

.pod-box {
  padding: 5px 10px;
  border-radius: 4px;
  text-align: center;
  flex: 1 1 40%;
  min-width: 80px;
  font-size: 0.9em;
}

.no-pods {
  font-size: 0.9em;
  font-style: italic;
}

/* Dynamic pod colors */
.pod-running {
  background-color: var(--green);
}
.pod-pending {
  background-color: var(--yellow);
}
.pod-failed {
  background-color: var(--red);
}
.pod-succeeded {
  background-color: var(--blue);
}
.pod-unknown {
  background-color: lightgrey;
}
</style>
