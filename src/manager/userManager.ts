import { WebSocket } from "ws";

enum WebRTCEvents {
    OFFER = "OFFER",
    ANSWER = "ANSWER",
    ICE_CANDIDATE = "ICE_CANDIDATE",
    USER_JOINED = "USER_JOINED",
    USER_LEFT = "USER_LEFT",
    USER_REQUEST = "USER_REQUEST",
    USER_REQUEST_ACCEPTED = "USER_REQUEST_ACCEPTED",
    USER_REQUEST_REJECTED = "USER_REQUEST_REJECTED",
}

interface Message {
    type: WebRTCEvents;
    sender: string;
    name: string;
    receiver: string;
}

export class UserManager {
    private users: WebSocket[];
    private waitingUsers: WebSocket[];
    private hostId: string;
    public GLOBAL_ID: number;

    constructor(hostId: string) {
        this.hostId = hostId;
        this.GLOBAL_ID = 0;
        this.users = [];
        this.waitingUsers = [];
    }

    private requestToJoin(socket: WebSocket) {
        this.waitingUsers.push(socket);
        const msg: Message = {
            type: WebRTCEvents.USER_REQUEST,
            sender: socket.id,
            name: socket.name,
            receiver: this.hostId,
        };

        this.sendMessage(msg);
        console.log(`User ${socket.id} requested to join`);
    }

    private joinMeeting(socket: WebSocket) {
        this.users.push(socket);
        if (this.users.length > 1) {
            this.sendMessage({
                type: WebRTCEvents.USER_REQUEST_ACCEPTED,
                sender: socket.id,
                receiver: socket.id,
                name: socket.name,
            });
            this.broadcast({
                type: WebRTCEvents.USER_JOINED,
                sender: socket.id,
                name: socket.name,
            });
        } else {
            this.sendMessage({
                type: WebRTCEvents.USER_REQUEST_ACCEPTED,
                sender: socket.id,
                receiver: socket.id,
                name: socket.name,
            });
        }
    }

    public addUser(socket: WebSocket) {
        this.onMessage(socket)
        socket.on('close', () => this.removeUser(socket.id));

        if (socket.id === this.hostId) {
            this.joinMeeting(socket);
        } else {
            this.requestToJoin(socket);
        }

    }

    private onMessage = (socket: WebSocket) => {
        socket.on('message', (msg) => {
            const parsedMsg = JSON.parse(msg.toString()) as Message;
            parsedMsg.sender = socket.id;
            parsedMsg.name = socket.name;

            switch (parsedMsg.type) {
                case WebRTCEvents.OFFER:
                case WebRTCEvents.ANSWER:
                case WebRTCEvents.ICE_CANDIDATE:
                    this.sendMessage(parsedMsg);
                    break;
                case WebRTCEvents.USER_REQUEST:
                    this.requestToJoin(socket);
                    break;
                case WebRTCEvents.USER_REQUEST_ACCEPTED:
                    const user = this.waitingUsers.find((user) => user.id === parsedMsg.receiver);
                    if (user) {
                        this.waitingUsers = this.waitingUsers.filter((u) => u.id !== user.id);
                        this.joinMeeting(user);
                    }
                    break;
                case WebRTCEvents.USER_REQUEST_REJECTED:
                    socket.close(400, 'User request rejected');
                    console.log(`User ${socket.id} rejected`);
                    break;
                case WebRTCEvents.USER_LEFT:
                    this.removeUser(socket.id);
                    break;
                default:
                    console.error(`Unknown message type: ${parsedMsg.type}`);
                    break;
            }
        });
    }

    private removeUser(id: string) {
        this.broadcast({
            type: WebRTCEvents.USER_LEFT,
            sender: id,
        });
        this.users = this.users.filter((user) => user.id !== id);
    }

    private broadcast(msg: Partial<Message>) {
        this.users.forEach((user) => user.id !== msg.sender && user.send(JSON.stringify(msg)));
    }

    private sendMessage(msg: Message) {
        const user = this.users.find((user) => user.id === msg.receiver);
        user?.send(JSON.stringify(msg));
    }
}