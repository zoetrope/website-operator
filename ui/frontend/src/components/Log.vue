<template>
  <v-row justify="center">
    <v-dialog
        v-model="dialog"
        width="800px"
    >
      <template v-slot:activator="{ on, attrs }">
        <v-btn
            color="primary"
            dark
            v-bind="attrs"
            v-on="on"
        >
          show
        </v-btn>
      </template>
      <v-card>
        <v-card-title>
          <span class="headline">Build Log: {{ name }}({{ namespace }})</span>
        </v-card-title>
        <v-card-text style="white-space:pre-wrap; word-wrap:break-word;">
          {{ log }}
        </v-card-text>
      </v-card>
    </v-dialog>
  </v-row>
</template>
<script>
import axios from 'axios';

export default {
  props: ['name', 'namespace'],
  data() {
    return {
      dialog: false,
      log: "",
    }
  },
  mounted() {
    axios
        .get('http://localhost:8080/api/v1/logs/' + this.namespace + "/" + this.name)
        .then(response => {
          console.log(response.data);
          (this.log = response.data)
        })
  }
}
</script>
