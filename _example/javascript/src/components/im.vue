<template>
  <div>
    <SentReceivedViewer
        :sent="sent"
        :received="received"
    />
    <div class="chat-input">
      <input type="text" class="form-control" v-model="inputMessage" @keydown.enter="sendMessage()">
      <button class="btn btn-primary" @click="sendMessage">Send</button>
      <button @click="openNewPage">open</button>
    </div>

  </div>
</template>

<script>
import SentReceivedViewer from "@/components/SentReceivedViewer.vue";

export default {
  props: {
    socket: {
      type: Object,
      default: null
    }
  },
  components: {
    SentReceivedViewer
  },
  data() {
    return {
      received: [],
      sent: [],
      inputMessage: "",
      isConnected: false,
      clientName: "ava",
      queryParams: {}

    };
  },
  methods: {
    sendMessage() {
      const params = new URLSearchParams(location.search)
      let name = params.get("name")
      if (name === "" || name === null) {
        name = this.clientName
      }

      if (!this.isConnected) {
        if (this.socket) {
          console.log("handshake sendMessage......")
          const message = {message: "handshake stream success", name: name};
          this.socket.requestStream({
            data: message,
            metadata: {
              trace: "123",
              method: "/hello/im/handshake",
              service: "api.hello",
              version: "v1.0.0"
            }
          }).subscribe({
            onComplete: () => {
              console.log("requestStream done");
              this.received.push("requestStream done");
            },
            onError: error => {
              console.log("got error with requestStream");
              console.error(error);
            },
            onNext: value => {
              // console.log("got next value in requestStream..");
              this.received.push(value.data);
            },
            // Nothing happens until `request(n)` is called
            onSubscribe: sub => {
              console.log("subscribe request Stream!");
              sub.request(1000000000);
              this.sent.push(message);
            }
          });

          this.isConnected = true;

        } else {
          console.log("im not connected...");
        }
      }

      if (!this.inputMessage) {
        return;
      }

      console.log("im sendMessage...")

      if (this.socket) {
        this.socket
            .requestResponse({
              data: {message: this.inputMessage, name: name},
              metadata: {
                trace: "123",
                method: "/hello/im/send",
                service: "api.hello",
                version: "v1.0.0",
              }
            }).subscribe({
          onComplete: data => {
            console.log("got response with requestResponse", data.data);
            this.received.push(data.data);
          },
          onError: error => {
            console.log("got error with requestResponse");
            console.error(error);
          },
          onSubscribe: cancel => {
            /* call cancel() to stop onComplete/onError */
          }
        });
      } else {
        console.log("not connected...");
      }
    },
    openNewPage() {
      const randomNumber = Math.floor(Math.random() * 10);
      const url = `?name=ava` + randomNumber;
      window.open(url, '_blank');
    },
  },
};
</script>

<style lang="scss" scoped>
</style>