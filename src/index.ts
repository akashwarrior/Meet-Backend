import { WebSocket } from 'ws';
import { UserManager } from './manager/userManager';
import { PrismaClient } from '../prisma/generated/prisma';
import { decode } from 'next-auth/jwt';
import http from 'http';
import url from 'url';
import dotenv from 'dotenv';
dotenv.config();

declare module 'ws' {
    interface WebSocket {
        id: string;
        name: string;
    }
}

declare module 'http' {
    interface IncomingMessage {
        data: {
            roomId: string;
            userId?: string;
            name: string;
            hostId: string;
        };
    }
}

const prisma = new PrismaClient()
const server = http.createServer();
const wss = new WebSocket.Server({ noServer: true });

function parseCookies(cookieHeader: string) {
    if (!cookieHeader) return null;
    const cookies = cookieHeader.split(';').map(cookie => cookie.trim());

    for (const cookie of cookies) {
        const [name, ...rest] = cookie.trim().split('=');
        if (name.includes('next-auth.session-token')) {
            return decodeURIComponent(rest.join('='));
        }
    }

    return null;
}

server.on('upgrade', async (request, socket, head) => {
    const parsedUrl = url.parse(request.url || "", true);
    const meetingId = parsedUrl.query.meetingId;
    const name = parsedUrl.query.name || 'unknown';
    const cookieHeader = request.headers['cookie'] || '';
    const sessionToken = parseCookies(cookieHeader);

    if (!meetingId) {
        socket.write('HTTP/1.1 400 Bad Request\r\n\r\n');
        socket.destroy();
        return;
    }

    const room = await prisma.meeting.findUnique({
        where: {
            id: meetingId as string,
        },
        select: {
            id: true,
            hostId: true,
        }
    });

    if (!room) {
        socket.write('HTTP/1.1 404 Invalid Room ID\r\n\r\n');
        socket.destroy();
        return;
    }

    const data: http.IncomingMessage['data'] = {
        roomId: room.id,
        hostId: room.hostId,
        name: name as string,
        userId: undefined,
    }

    if (sessionToken) {
        const decoded = await decode({
            token: sessionToken,
            secret: process.env.NEXTAUTH_SECRET as string,
        });
        if (decoded) {
            data.userId = decoded.sub;
            data.name = decoded.name || data.name;
        }
    }

    request.data = data;

    // Accept WebSocket upgrade
    wss.handleUpgrade(request, socket, head, (ws) => {
        wss.emit('connection', ws, request);
    });
});

const map = new Map<string, UserManager>();

wss.on('connection', async (ws: WebSocket, req: http.IncomingMessage) => {
    const { roomId, userId, hostId, name } = req.data;
    let manager;
    if (!map.has(roomId)) {
        manager = new UserManager(hostId);
        map.set(roomId, manager);
    } else {
        manager = map.get(roomId)!!;
    }
    ws.id = userId ? userId : `${roomId}-${manager.GLOBAL_ID++}`;
    ws.name = name;
    manager.addUser(ws);
});


server.listen(8080, () => {
    console.log('Server running on http://localhost:8080');
});