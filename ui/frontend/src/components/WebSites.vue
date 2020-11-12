<template>
  <v-simple-table>
    <template v-slot:default>
      <thead>
      <tr>
        <th class="text-left">
          Name
        </th>
        <th class="text-left">
          Namespace
        </th>
        <th class="text-left">
          URL
        </th>
        <th class="text-left">
          Branch
        </th>
        <th class="text-left">
          Ready
        </th>
        <th class="text-left">
          Revision
        </th>
        <th class="text-left">
          Build Log
        </th>
      </tr>
      </thead>
      <tbody>
      <tr
          v-for="item in websites"
          :key="item.name"
      >
        <td>{{ item.name }}</td>
        <td>{{ item.namespace }}</td>
        <td>{{ item.url }}</td>
        <td>{{ item.branch }}</td>
        <td>{{ item.ready }}</td>
        <td>{{ item.revision }}</td>
        <td>
          <Log v-bind:name="item.name" v-bind:namespace="item.namespace"/>
        </td>
      </tr>
      </tbody>
    </template>
  </v-simple-table>
</template>
<script>
import axios from 'axios';
import Log from "./Log"

export default {
  components: {Log},
  data() {
    return {
      websites: [],
    }
  },
  mounted() {
    axios
        .get('http://localhost:8080/api/v1/websites')
        .then(response => {
          console.log(response.data);
          (this.websites = response.data)
        })
  }
}
</script>
