<template>
  <v-app>
    <v-app-bar app color="primary" dark >
      <div class="d-flex align-center">
        WebSite Operator
      </div>
    </v-app-bar>
    <v-main>
      <v-container>
        <WebSites/>
      </v-container>
    </v-main>
  </v-app>
</template>

<script>
import WebSites from "./components/WebSites"

export default {
  name: 'App',

  components: {
    WebSites,
  },

  data: () => ({
    //
  }),
};
</script>
