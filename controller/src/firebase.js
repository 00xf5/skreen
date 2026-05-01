import { initializeApp } from 'firebase/app'
import {
  getAuth,
  GoogleAuthProvider,
  signInWithPopup,
  signInWithEmailAndPassword,
  createUserWithEmailAndPassword,
  signOut,
  onAuthStateChanged,
} from 'firebase/auth'

const firebaseConfig = {
  apiKey: 'AIzaSyDuEQ9pb_SP5QjbF3XTBJ66e9teYlo3n9k',
  authDomain: 'skreen-63b4c.firebaseapp.com',
  projectId: 'skreen-63b4c',
  storageBucket: 'skreen-63b4c.firebasestorage.app',
  messagingSenderId: '33112093797',
  appId: '1:33112093797:web:9ccf8c9ef48d50cd7f828c',
  measurementId: 'G-3KJTP7RD7E',
}

const app = initializeApp(firebaseConfig)
export const auth = getAuth(app)
export const googleProvider = new GoogleAuthProvider()

export {
  signInWithPopup,
  signInWithEmailAndPassword,
  createUserWithEmailAndPassword,
  signOut,
  onAuthStateChanged,
}
